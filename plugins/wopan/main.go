// Wopan is a native Flowpan storage provider, not a separate HTTP service.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
	"flowpan/sdk/pluginwasm"
)

const rootID = "wopan:root"

type credentials struct {
	Access  string    `json:"access_token"`
	Refresh string    `json:"refresh_token"`
	UID     string    `json:"uid"`
	Seed    string    `json:"seed,omitempty"`
	Expires time.Time `json:"expires_at,omitempty"`
}
type adapter struct {
	client  *wopan.Client
	call    *pluginapi.StorageCall
	creds   credentials
	rotated bool
}
type hostTransport struct{}

func (hostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, 65537))
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(body) > 65536 {
			return nil, errors.New("联通云盘元数据请求过大")
		}
	}
	headers := map[string]string{}
	for k := range req.Header {
		headers[k] = req.Header.Get(k)
	}
	var response pluginapi.StorageHTTPResponse
	err := pluginwasm.Call("http.fetch", map[string]any{"url": req.URL.String(), "method": req.Method, "headers": headers, "body": string(body)}, &response)
	if err != nil {
		return nil, err
	}
	if err = req.Context().Err(); err != nil {
		return nil, err
	}
	h := http.Header{}
	for k, v := range response.Headers {
		h.Set(k, v)
	}
	if response.URL != "" {
		if u, e := url.Parse(response.URL); e == nil {
			copy := req.Clone(req.Context())
			copy.URL = u
			req = copy
		}
	}
	return &http.Response{StatusCode: response.Status, Header: h, Body: io.NopCloser(strings.NewReader(response.Body)), Request: req}, nil
}

func main() {
	log.SetOutput(io.Discard)
	pluginwasm.Serve(pluginwasm.Handlers{Storage: storage, Health: func(pluginwasm.Invocation) (pluginapi.Result, error) { return pluginapi.Result{Message: "ready"}, nil }})
}
func storage(in pluginwasm.Invocation) (pluginapi.Result, error) {
	if in.Request.Storage == nil {
		return pluginapi.Result{}, errors.New("缺少网盘调用上下文")
	}
	return execute(context.Background(), in.Request.Storage, &http.Client{Transport: hostTransport{}})
}
func setting(c *pluginapi.StorageCall, key string) string {
	v, _ := c.Settings[key].(string)
	return strings.TrimSpace(v)
}
func seed(c *pluginapi.StorageCall) string {
	b, _ := json.Marshal([]string{setting(c, "access_token"), setting(c, "refresh_token")})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func newAdapter(c *pluginapi.StorageCall, h *http.Client) (*adapter, error) {
	a := &adapter{call: c}
	if len(c.Credentials) > 0 && string(c.Credentials) != "null" {
		if err := json.Unmarshal(c.Credentials, &a.creds); err != nil {
			return nil, errors.New("联通云盘登录凭据无效")
		}
	}
	supplied := seed(c)
	if a.creds.Access == "" || (c.Operation == "configure" || c.Operation == "login.begin") && supplied != a.creds.Seed && (setting(c, "access_token") != "" || setting(c, "refresh_token") != "") {
		a.creds = credentials{Access: setting(c, "access_token"), Refresh: setting(c, "refresh_token"), Seed: supplied}
	}
	device := sha256.Sum256([]byte("flowpan:wopan:" + c.Account))
	a.client = wopan.New(wopan.Config{HTTP: h, AccessToken: a.creds.Access, RefreshToken: a.creds.Refresh, UserAgent: wopan.DefaultUserAgent, DeviceNo: hex.EncodeToString(device[:16]), OnTokens: func(access, refresh string, expires int64) {
		a.creds.Access, a.creds.Refresh = access, refresh
		a.creds.Expires = time.Time{}
		if expires > 0 && expires <= 366*24*3600 {
			a.creds.Expires = time.Now().Add(time.Duration(expires) * time.Second)
		}
		a.rotated = true
	}})
	return a, nil
}
func execute(ctx context.Context, c *pluginapi.StorageCall, h *http.Client) (pluginapi.Result, error) {
	a, err := newAdapter(c, h)
	if err != nil {
		return pluginapi.Result{}, err
	}
	value, err := a.dispatch(ctx)
	result := pluginwasm.StorageResult(value)
	// Return rotations even when a later metadata operation fails. The host saves
	// them under the same account/revision before it reports the operation error.
	if a.rotated && c.Operation != "configure" && c.Operation != "verify" && c.Operation != "refresh" && !strings.HasPrefix(c.Operation, "login.") && a.creds.UID != "" {
		uid, _, identityErr := a.client.QueryUser(ctx)
		if identityErr != nil || uid != a.creds.UID {
			return pluginapi.Result{}, errors.New("联通云盘续期后账号校验失败，请重新登录")
		}
		token, refresh := a.client.Tokens()
		a.creds.Access, a.creds.Refresh = token, refresh
		b, _ := json.Marshal(a.creds)
		result.Data["storage_credentials"] = pluginapi.StorageCredentials{Credentials: b, ExpiresAt: a.creds.Expires}
	}
	if err != nil {
		message := err.Error()
		for _, s := range []string{a.creds.Access, a.creds.Refresh, setting(c, "access_token"), setting(c, "refresh_token")} {
			if s != "" {
				message = strings.ReplaceAll(message, s, "[已隐藏]")
			}
		}
		result.Failed = true
		result.Message = message
	}
	return result, nil
}
func (a *adapter) ready(ctx context.Context) error {
	token, refresh := a.client.Tokens()
	if len(token) < 16 {
		if refresh == "" {
			return errors.New("请扫码登录联通云盘")
		}
		return a.client.Refresh(ctx)
	}
	return nil
}
func (a *adapter) auth(ctx context.Context, refresh bool) (pluginapi.StorageAuth, error) {
	if err := a.ready(ctx); err != nil {
		return pluginapi.StorageAuth{}, err
	}
	if refresh && !a.rotated {
		_, rt := a.client.Tokens()
		if rt != "" {
			if err := a.client.Refresh(ctx); err != nil {
				return pluginapi.StorageAuth{}, err
			}
		}
	}
	uid, name, err := a.client.QueryUser(ctx)
	if err != nil {
		return pluginapi.StorageAuth{}, err
	}
	// Verify/refresh never silently binds a different account. Configure and a
	// newly confirmed QR login deliberately establish a new identity.
	if (a.call.Operation == "verify" || a.call.Operation == "refresh") && a.creds.UID != "" && uid != a.creds.UID {
		return pluginapi.StorageAuth{}, errors.New("联通云盘账号已变化，请重新登录")
	}
	token, rt := a.client.Tokens()
	a.creds.Access, a.creds.Refresh, a.creds.UID = token, rt, uid
	if a.creds.Seed == "" {
		a.creds.Seed = seed(a.call)
	}
	b, _ := json.Marshal(a.creds)
	auth := pluginapi.StorageAuth{State: "confirmed", Identity: uid, RootID: rootID, Name: name, Credentials: b, ExpiresAt: a.creds.Expires}
	// Quota is supplementary: it must not turn a successful login into a failure.
	if quota, e := a.client.Storage(ctx); e == nil {
		auth.SizeUsed = fmt.Sprint(quota.UsedSpace)
		auth.SizeTotal = fmt.Sprint(quota.AllSpace)
	}
	// Storage() can refresh too; return the last pair, not the initial snapshot.
	token, rt = a.client.Tokens()
	a.creds.Access, a.creds.Refresh = token, rt
	auth.Credentials, _ = json.Marshal(a.creds)
	auth.ExpiresAt = a.creds.Expires
	return auth, nil
}

type qrSession struct {
	UUID    string `json:"uuid"`
	Image   string `json:"image"`
	Started int64  `json:"started"`
}

func (a *adapter) dispatch(ctx context.Context) (any, error) {
	c := a.call
	p := c.Params
	switch c.Operation {
	case "share.resolve", "share.save", "share.status":
		return a.share(ctx)
	case "login.begin":
		if p.Method != "qrcode" {
			return nil, errors.New("联通云盘登录方式无效")
		}
		qr, err := a.client.QRStart(ctx)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(qr.Image, "data:image/png;base64,") || len(qr.Image) > 200000 {
			return nil, errors.New("联通云盘二维码无效")
		}
		b, _ := json.Marshal(qrSession{qr.UUID, qr.Image, time.Now().Unix()})
		return pluginapi.StorageAuth{State: "waiting", QRCode: qr.Image, Session: b}, nil
	case "login.poll":
		var session qrSession
		if json.Unmarshal(c.Session, &session) != nil || session.UUID == "" || len(session.UUID) > 2048 || session.Started <= 0 || session.Started > time.Now().Unix()+30 {
			return nil, errors.New("联通云盘扫码会话无效")
		}
		if time.Now().Unix()-session.Started > 300 {
			return pluginapi.StorageAuth{State: "expired"}, nil
		}
		state, token, rt, err := a.client.QRState(ctx, session.UUID)
		if err != nil {
			return nil, err
		}
		switch state {
		case 1, 2:
			status := "waiting"
			if state == 2 {
				status = "scanned"
			}
			return pluginapi.StorageAuth{State: status, Session: c.Session, QRCode: session.Image}, nil
		case 3:
			if len(token) < 16 {
				return nil, errors.New("联通云盘扫码未返回有效 Token")
			}
			a.creds = credentials{Access: token, Refresh: rt, Seed: seed(c)}
			_ = a.client.SetTokens(token, rt)
			return a.auth(ctx, false)
		case 4:
			return pluginapi.StorageAuth{State: "expired"}, nil
		default:
			return nil, errors.New("联通云盘返回未知扫码状态")
		}
	case "configure", "verify", "refresh":
		return a.auth(ctx, c.Operation == "refresh")
	}
	if err := a.ready(ctx); err != nil {
		return nil, err
	}
	if a.creds.UID == "" {
		return nil, errors.New("请先验证联通云盘账号")
	}
	switch c.Operation {
	case "list":
		return a.list(ctx, p)
	case "stat":
		return a.stat(ctx, p.ID)
	case "read", "link":
		ref, err := decodeRef(p.ID)
		if err != nil {
			return nil, err
		}
		if ref.Dir {
			return nil, errors.New("目录不支持播放")
		}
		fid := ref.FID
		if fid == "" {
			e, err := a.find(ctx, ref.Parent, func(e wopan.Entry) bool { return e.ID == ref.ID })
			if err != nil {
				return nil, err
			}
			fid = e.FID
		}
		link, err := a.client.DownloadURL(ctx, fid, wopan.SpacePersonal)
		if err != nil {
			return nil, err
		}
		// Signed URLs stay intact. Account access/refresh tokens are never sent to a CDN.
		return pluginapi.StorageLink{URL: link}, nil
	case "mkdir":
		parent, err := decodeRef(p.ParentID)
		if err != nil || !parent.Dir {
			return nil, errors.New("请选择联通云盘目录")
		}
		if err = validName(p.Name); err != nil {
			return nil, err
		}
		if err = waitWrite(ctx); err != nil {
			return nil, err
		}
		id, err := a.client.Mkdir(ctx, wopan.SpacePersonal, parent.ID, p.Name, "")
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, errors.New("联通云盘建目录未返回标识，请检查远端")
		}
		return entry(wopan.Entry{ID: id, Name: p.Name, IsDir: true}, parent), nil
	case "rename":
		ref, err := decodeRef(p.ID)
		if err != nil {
			return nil, err
		}
		if ref.ID == "0" {
			return nil, errors.New("根目录不支持重命名")
		}
		if err = validName(p.Name); err != nil {
			return nil, err
		}
		current, err := a.find(ctx, ref.Parent, func(e wopan.Entry) bool { return e.ID == ref.ID })
		if err != nil {
			return nil, err
		}
		if err = waitWrite(ctx); err != nil {
			return nil, err
		}
		_, err = a.client.Rename(ctx, wopan.SpacePersonal, ref.ID, p.Name, ref.Dir, "", "", current.FileVersion)
		return nil, err
	case "delete", "move":
		return nil, a.mutate(ctx, c.Operation, p)
	case "upload":
		return a.upload(ctx, p)
	default:
		return nil, errors.New("联通云盘操作未开放")
	}
}

// Each WASM invocation has a fresh client. Preserve the supplied SDK's 800ms
// write pacing across invocations; the host serializes each account's calls.
func waitWrite(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(800 * time.Millisecond):
		return nil
	}
}

func validName(name string) error {
	if strings.TrimSpace(name) == "" || len(name) > 1024 || strings.ContainsAny(name, "/\\\x00\r\n") || name == "." || name == ".." {
		return errors.New("文件名称无效")
	}
	return nil
}
