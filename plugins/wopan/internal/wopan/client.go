package wopan

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultBaseURL    = "https://panservice.mail.wo.cn"
	DefaultClientID   = "1001000209"
	DefaultUploadHost = "https://tjupload.pan.wo.cn"
	quickTransferAPI  = "https://b.smartont.net/openapi/transfer/quickTransfer"

	// Windows 桌面端 prod secret；其它端见 README（mac/cloud/ticket 各不相同）
	DefaultSecretKey = "s72g2VY46ahlK588"

	pcVersion = "2.8.13"

	// 与 2.8.13 客户端一致：Electron 22.3.27 / Chrome 108.0.5359.215。
	// packaged app 会把 <appName>/<appVersion> 拼进 UA（package.json name=wocloud），
	// 不是浏览器那种 Chrome-only 形式。
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"wocloud/2.8.13 Chrome/108.0.5359.215 Electron/22.3.27 Safari/537.36"
)

// ErrNeedLogin 表示必须重新扫码/密码登录，调用方应回 401 而不是重试
var ErrNeedLogin = errors.New("wopan: 没有 refresh_token，需要重新扫码登录")

// IsNeedLogin 两条通道的“未登录”码不一样：dispatcher 用 1001，REST 用 10002404（实测）
func IsNeedLogin(code string) bool { return code == "1001" || code == "10002404" }

// APIError 保留服务端错误码，方便上层区分"没权限"和"网络错"
type APIError struct {
	Code string
	Desc string
}

func (e *APIError) Error() string { return "wopan: " + e.Code + " " + e.Desc }

type Config struct {
	BaseURL   string
	ClientID  string
	SecretKey string
	DeviceNo  string // 设备号；留空则用本机稳定派生值（真实客户端是本地硬件指纹）
	UserAgent string // 留空用 DefaultUserAgent

	// MinWriteGap 写操作最小间隔。实测连续 ~20 次写（建/传/删）会触发服务端限流，
	// 之后一段时间全回 9999/502；留 0 用默认 800ms，负值表示完全不限速。
	MinWriteGap  time.Duration
	AccessToken  string
	RefreshToken string
	TokenFile    string // 非空则自动读写 {access_token, refresh_token}
	UploadHost   string
	HTTP         *http.Client
	OnTokens     func(access, refresh string, expiresIn int64)
}

type Client struct {
	cfg  Config
	http *http.Client

	deviceNo string

	writeMu   sync.Mutex
	lastWrite time.Time
	writeGap  time.Duration

	mu      sync.RWMutex
	access  string
	refresh string

	// refreshMu 把续期串行化：并发请求同时拿到 1001 时，只允许一个真正去换 token
	refreshMu sync.Mutex

	// 续期台账：给 /api/token/status 看，用于确认“真的续了”
	renewCount int
	renewAt    time.Time
	renewErr   string
}

type tokenFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultClientID
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = DefaultSecretKey
	}
	if cfg.UploadHost == "" {
		cfg.UploadHost = DefaultUploadHost
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 90 * time.Second}
	}
	gap := cfg.MinWriteGap
	if gap == 0 {
		gap = 800 * time.Millisecond
	}
	c := &Client{cfg: cfg, http: cfg.HTTP, access: cfg.AccessToken, refresh: cfg.RefreshToken,
		deviceNo: deriveDeviceNo(cfg.DeviceNo), writeGap: gap}
	if c.access == "" && cfg.TokenFile != "" {
		if b, err := os.ReadFile(cfg.TokenFile); err == nil {
			var t tokenFile
			if json.Unmarshal(b, &t) == nil {
				c.access, c.refresh = t.AccessToken, t.RefreshToken
			}
		}
	}
	return c
}

// throttleWrite 写操作节流：保证两次写之间至少 writeGap，避免触发服务端频率限制。
func (c *Client) throttleWrite(ctx context.Context) error {
	if c.writeGap < 0 {
		return nil
	}
	c.writeMu.Lock()
	wait := time.Until(c.lastWrite.Add(c.writeGap))
	if wait > 0 {
		select {
		case <-ctx.Done():
			c.writeMu.Unlock()
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	c.lastWrite = time.Now()
	c.writeMu.Unlock()
	return nil
}

// DeviceNo 设备号：优先用配置；否则用本机稳定派生值。
// 真实客户端取的是本地硬件指纹（win_lib.node），这里退化成 hostname+os 的 hash——
// 服务端似乎只把它当一个不透明字符串（未带也能上传），所以够用。
func (c *Client) DeviceNo() string { return c.deviceNo }

func deriveDeviceNo(configured string) string {
	if configured != "" {
		return configured
	}
	host, _ := os.Hostname()
	sum := sha1.Sum([]byte("wopan-go|" + host + "|" + runtime.GOOS + "|" + runtime.GOARCH))
	return hex.EncodeToString(sum[:])[:32]
}
func (c *Client) Tokens() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.access, c.refresh
}

// SetTokens 更新并落盘（0600）
func (c *Client) SetTokens(access, refresh string) error {
	c.mu.Lock()
	c.access, c.refresh = access, refresh
	path := c.cfg.TokenFile
	c.mu.Unlock()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(tokenFile{access, refresh}, "", "  ")
	return os.WriteFile(path, b, 0o600)
}

func (c *Client) keyFor(channel string) string {
	if channel == "api-user" {
		return c.cfg.SecretKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.access == "" {
		return c.cfg.SecretKey
	}
	return c.access // suitableKey 会截到 16 字节
}

// Refresh 用 refreshToken 换新 token（服务端 key: AppRefreshToken）
func (c *Client) Refresh(ctx context.Context) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	return c.refreshLocked(ctx)
}

// renew 只换一次：stale 是请求发出时的 token，若已被别人换过就直接返回
func (c *Client) renew(ctx context.Context, stale string) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	if stale != "" {
		c.mu.RLock()
		cur := c.access
		c.mu.RUnlock()
		if cur != stale {
			return nil
		}
	}
	return c.refreshLocked(ctx)
}

// TokenFile 返回当前 token 文件路径（供状态查询/运维用）
func (c *Client) TokenFile() string { return c.cfg.TokenFile }

// RenewStat 续期台账：调用方可据此判断“到底有没有续”
type RenewStat struct {
	Count     int       `json:"count"`
	LastAt    time.Time `json:"last_at"`
	LastOK    bool      `json:"last_ok"`
	LastError string    `json:"last_error,omitempty"`
}

func (c *Client) RenewStat() RenewStat {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return RenewStat{Count: c.renewCount, LastAt: c.renewAt, LastOK: c.renewErr == "", LastError: c.renewErr}
}

func (c *Client) markRenew(err error) {
	c.mu.Lock()
	c.renewCount++
	c.renewAt = time.Now()
	if err == nil {
		c.renewErr = ""
	} else {
		c.renewErr = err.Error()
	}
	c.mu.Unlock()
	if err == nil {
		log.Printf("[wopan] token 续期成功")
	} else {
		log.Printf("[wopan] token 续期失败: %v", err)
	}
}

func mask(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:8]
}

func (c *Client) refreshLocked(ctx context.Context) error {
	err := c.doRefresh(ctx)
	c.markRenew(err)
	return err
}

func (c *Client) doRefresh(ctx context.Context) error {
	c.mu.RLock()
	rt := c.refresh
	c.mu.RUnlock()
	if rt == "" {
		return ErrNeedLogin
	}
	data, err := c.dispatcher(ctx, "api-user", "AppRefreshToken", map[string]any{
		"clientId":     c.cfg.ClientID,
		"clientSecret": c.cfg.SecretKey,
		"refreshToken": rt,
	}, false)
	if err != nil {
		return err
	}
	var r struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("wopan: 续期响应解析失败: %w", err)
	}
	if len(r.AccessToken) < 16 {
		return errors.New("wopan: 续期未返回 access_token")
	}
	if r.RefreshToken == "" {
		r.RefreshToken = rt
	}
	if err := c.SetTokens(r.AccessToken, r.RefreshToken); err != nil {
		return err
	}
	if c.cfg.OnTokens != nil {
		c.cfg.OnTokens(r.AccessToken, r.RefreshToken, r.ExpiresIn)
	}
	return nil
}

// dispatcher 调用 /wohome/dispatcher 或 /api-user/dispatcher
func (c *Client) dispatcher(ctx context.Context, channel, key string, params map[string]any, allowRefresh bool) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	if _, ok := params["clientId"]; !ok {
		params["clientId"] = c.cfg.ClientID
	}
	if key == "AppQueryUser" {
		params["accessToken"], _ = c.Tokens()
	}
	plain, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	enc, err := aesEncrypt(string(plain), c.keyFor(channel))
	if err != nil {
		return nil, err
	}
	resTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	reqSeq := strconv.Itoa(100000 + rand.Intn(89999))
	path := "/wohome/dispatcher"
	// wohome: body={param,secret:true}；api-user: body={param,clientId}（clientId 在顶层明文，否则 1000 参数错误）
	body := map[string]any{"param": enc, "secret": true}
	if channel == "api-user" {
		path = "/api-user/dispatcher"
		body = map[string]any{"param": enc, "clientId": c.cfg.ClientID}
	}
	payload := map[string]any{
		"header": map[string]any{
			"key": key, "resTime": resTime, "reqSeq": reqSeq,
			"channel": channel, "version": "",
			"sign": signSum(key, resTime, reqSeq, channel, ""),
		},
		"body": body,
	}
	// 必须取发请求前的 token：并发时别人换过就靠它判定“已续期，不需再换”
	c.mu.RLock()
	tokBefore := c.access
	c.mu.RUnlock()
	raw, err := c.postJSON(ctx, path, payload)
	if err != nil {
		return nil, err
	}
	var env struct {
		RSP struct {
			Code string          `json:"RSP_CODE"`
			Desc string          `json:"RSP_DESC"`
			Data json.RawMessage `json:"DATA"`
		} `json:"RSP"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("wopan: 响应不是预期 JSON: %w", err)
	}
	code := env.RSP.Code
	if IsNeedLogin(code) && allowRefresh {
		if err := c.renew(ctx, tokBefore); err != nil {
			return nil, err
		}
		return c.dispatcher(ctx, channel, key, params, false)
	}
	// 9999 系统异常实测连读取都会偶发（不限写），首次调用时退避重试一次
	if code == "9999" && allowRefresh && dispatcherRead(key) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
		return c.dispatcher(ctx, channel, key, params, false)
	}
	if code != "0000" {
		return nil, &APIError{Code: code, Desc: env.RSP.Desc}
	}
	return c.openData(channel, env.RSP.Data)
}

// Only reads can replay a system-error response. A write may already have
// completed remotely even when its receipt says 9999.
func dispatcherRead(key string) bool {
	switch key {
	case "QueryAllFiles", "AppQueryUser", "GetDownloadUrlV2":
		return true
	}
	return false
}

// openData：DATA 可能是加密字符串，也可能是明文对象
func (c *Client) openData(channel string, data json.RawMessage) (json.RawMessage, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return json.RawMessage("{}"), nil
	}
	if data[0] != '"' {
		return data, nil
	}
	var enc string
	if err := json.Unmarshal(data, &enc); err != nil {
		return nil, err
	}
	plain, err := aesDecrypt(enc, c.keyFor(channel))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(plain), nil
}

// restJSON 走带 X-YP-Access-Token 的普通 REST 接口（/api/bff/*、/wohome/pass/*、/wohome/open/*）
func (c *Client) restJSON(ctx context.Context, method, path string, body any, allowRefresh bool) (json.RawMessage, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	c.mu.RLock()
	tok := c.access
	c.mu.RUnlock()
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("accesstoken", tok)
	req.Header.Set("Access-Token", tok)
	req.Header.Set("X-YP-Access-Token", tok)
	req.Header.Set("X-YP-Client-Id", c.cfg.ClientID)
	req.Header.Set("Client-Id", c.cfg.ClientID)
	req.Header.Set("pc-version", pcVersion)
	req.Header.Set("pc-os", "win")
	req.Header.Set("Sys-Version", "Android/10")
	req.Header.Set("platform", "4")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wopan: HTTP %d: %s", resp.StatusCode, truncate(raw, 300))
	}
	var env struct {
		Meta struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"meta"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("wopan: 响应不是预期 JSON: %w", err)
	}
	if IsNeedLogin(env.Meta.Code) && allowRefresh {
		if err := c.renew(ctx, tok); err != nil {
			return nil, err
		}
		return c.restJSON(ctx, method, path, body, false)
	}
	if env.Meta.Code != "200" {
		return nil, &APIError{Code: env.Meta.Code, Desc: env.Meta.Message}
	}
	if len(env.Result) == 0 {
		return json.RawMessage("{}"), nil
	}
	return env.Result, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body any) (json.RawMessage, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Origin", "https://pan.wo.cn")
	req.Header.Set("Referer", "https://pan.wo.cn/")
	c.mu.RLock()
	tok := c.access
	c.mu.RUnlock()
	req.Header.Set("accesstoken", tok)
	req.Header.Set("Access-Token", tok)
	req.Header.Set("X-YP-Access-Token", tok)
	req.Header.Set("X-YP-Client-Id", c.cfg.ClientID)
	req.Header.Set("Client-Id", c.cfg.ClientID)
	req.Header.Set("pc-version", pcVersion)
	req.Header.Set("pc-os", "win")
	req.Header.Set("Sys-Version", "Android/10")
	req.Header.Set("platform", "4")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wopan: HTTP %d: %s", resp.StatusCode, truncate(raw, 300))
	}
	return raw, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
