package wopan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const shareClientSecret = "XFmi9GS2hzk98jGX" // Public H5 ClientSecret, not an account credential.
type ShareFile struct {
	ID       string `json:"id"`
	FileID   string `json:"fileId,omitempty"`
	Name     string `json:"fileName"`
	Type     any    `json:"type"`
	FileType any    `json:"fileType,omitempty"`
}
type Share struct {
	ID, Code, ClientID string
	Files              []ShareFile
}

// Resolve only follows the official short-link redirect. The APP suffix is not
// a password. ShareId and shareCode come from the official H5 destination.
func (c *Client) ResolveShare(ctx context.Context, raw, password string) (Share, error) {
	var out Share
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return out, errors.New("联通分享链接无效")
	}
	if u.Host == "pan.wo.cn" && strings.HasPrefix(u.Path, "/s/") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return out, err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		res, err := c.http.Do(req)
		if err != nil {
			return out, errors.New("联通分享短链接解析失败")
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 8192))
		if res.StatusCode != 200 || res.Request == nil {
			return out, errors.New("联通分享短链接响应无效")
		}
		u = res.Request.URL
	}
	if u.Host != "panservice.mail.wo.cn" || u.Path != "/h5/wocloudshare/" {
		return out, errors.New("联通分享跳转地址无效")
	}
	out.ID, out.Code, out.ClientID = u.Query().Get("shareId"), u.Query().Get("shareCode"), u.Query().Get("clientId")
	if password != "" {
		out.Code = password
	}
	if out.ClientID == "" {
		out.ClientID = "1001000035"
	}
	if len(out.ID) != 32 || len(out.Code) > 32 {
		return out, errors.New("联通分享标识或提取码无效")
	}
	data, err := c.shareRequest(ctx, "ShareFileDetail", out.ClientID, map[string]any{"shareId": out.ID, "shareCode": out.Code}, true)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &out.Files); err != nil {
		return out, errors.New("联通分享文件响应无效")
	}
	if len(out.Files) == 0 {
		return out, errors.New("联通分享没有可转存的文件")
	}
	for i := range out.Files {
		f := &out.Files[i]
		if f.ID == "" {
			f.ID = f.FileID
		}
		if f.Type == nil {
			f.Type = f.FileType
		}
		if f.ID == "" || f.Name == "" {
			return out, errors.New("联通分享文件标识无效")
		}
	}
	return out, nil
}

func (c *Client) ReceiveShare(ctx context.Context, share Share, target string) error {
	if share.ID == "" || target == "" || len(share.Files) == 0 {
		return errors.New("联通转存参数无效")
	}
	dirs, files := []string{}, []string{}
	for _, f := range share.Files {
		if fmt.Sprint(f.Type) == "0" {
			dirs = append(dirs, f.ID)
		} else {
			files = append(files, f.ID)
		}
	}
	_, err := c.shareRequest(ctx, "ReceiveShare", share.ClientID, map[string]any{"clientId": share.ClientID, "shareId": share.ID, "spaceType": "-1", "targetDirId": target, "dirList": dirs, "fileList": files, "uphold": 0}, false)
	return err // Writes, including lost receipts, are never replayed.
}
func (c *Client) shareRequest(ctx context.Context, key, clientID string, param map[string]any, public bool) (json.RawMessage, error) {
	secret, kind := shareClientSecret, "ClientSecret"
	if !public {
		secret, _ = c.Tokens()
		kind = "TokenSecret"
		if len(secret) < 16 {
			return nil, ErrNeedLogin
		}
	}
	plain, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	encrypted, err := aesEncrypt(string(plain), secret)
	if err != nil {
		return nil, err
	}
	now, seq := strconv.FormatInt(time.Now().UnixMilli(), 10), strconv.Itoa(100000+rand.Intn(89999))
	body := map[string]any{"param": encrypted, "secret": true, "secretType": kind}
	if public {
		body["clientId"] = clientID
	}
	payload := map[string]any{"header": map[string]any{"key": key, "resTime": now, "reqSeq": seq, "channel": "100002", "version": "", "sign": signSum(key, now, seq, "100002", "")}, "body": body}
	raw, err := c.postJSON(ctx, "/wohome/dispatcher", payload)
	if err != nil {
		return nil, err
	}
	var env struct {
		RSP struct {
			Code       string          `json:"RSP_CODE"`
			Desc       string          `json:"RSP_DESC"`
			Data       json.RawMessage `json:"DATA"`
			Secret     string          `json:"SECRET"`
			SecretType string          `json:"SECRET_TYPE"`
		} `json:"RSP"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return nil, errors.New("联通分享响应格式无效")
	}
	if env.RSP.Code != "0000" {
		return nil, &APIError{Code: env.RSP.Code, Desc: env.RSP.Desc}
	}
	data := env.RSP.Data
	if len(data) > 0 && data[0] == '"' {
		var value string
		if json.Unmarshal(data, &value) != nil {
			return nil, errors.New("联通分享响应无效")
		}
		data, err = aesDecrypt(value, secret)
	}
	return data, err
}
