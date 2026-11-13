package wopan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// QueryUser binds the account to the server UID, not a token hash or a label.
// Protocol reference: OpenListTeam/wopan-sdk-go api-user.go AppQueryUser.
func (c *Client) QueryUser(ctx context.Context) (id, name string, err error) {
	token, _ := c.Tokens()
	data, err := c.dispatcher(ctx, "api-user", "AppQueryUser", map[string]any{"accessToken": token, "clientSecret": c.cfg.SecretKey}, true)
	if err != nil {
		return "", "", err
	}
	var v struct {
		ID   string `json:"userId"`
		Name string `json:"userName"`
	}
	if err = json.Unmarshal(data, &v); err != nil {
		return "", "", err
	}
	if v.ID == "" {
		return "", "", errors.New("联通云盘未返回账号标识")
	}
	return v.ID, v.Name, nil
}

// ListPage makes exactly one provider listing call. No recursive traversal and
// no automatic all-page aggregation at the ABI boundary.
func (c *Client) ListPage(ctx context.Context, parent string, page, size, sort int) ([]Entry, bool, error) {
	if page < 0 || size < 1 || size > 500 {
		return nil, false, errors.New("联通云盘分页参数无效")
	}
	raw, err := c.dispatcher(ctx, "wohome", "QueryAllFiles", map[string]any{"spaceType": SpacePersonal, "parentDirectoryId": parent, "pageNum": page, "pageSize": size, "sortRule": sort, "deviceNo": c.DeviceNo()}, true)
	if err != nil {
		return nil, false, err
	}
	entries, more, err := c.parseFiles(raw)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if _, explicit := fields["hasNext"]; !explicit {
		more = len(entries) == size
	}
	return entries, more, err
}

// QuickTransfer preserves an unknown receipt as an error; a timed-out quick
// transfer is not permission to create another file via a full upload.
func (c *Client) QuickTransfer(ctx context.Context, sha, name, parent string, size int64, batch string) (string, bool, error) {
	b, _ := json.Marshal(map[string]any{"sha256": sha, "fileSize": size, "fileName": name, "fileType": fileTypeOf(name), "batchNo": batch, "spaceType": SpacePersonal, "directoryId": parent, "fileModificationTime": time.Now().UnixMilli()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, quickTransferAPI, bytes.NewReader(b))
	if err != nil {
		return "", false, err
	}
	token, _ := c.Tokens()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("access-token", token)
	req.Header.Set("X-YP-Access-Token", token)
	res, err := c.http.Do(req)
	if err != nil {
		return "", false, errors.New("联通云盘秒传回执未确认，请检查远端后重试")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", false, fmt.Errorf("联通云盘秒传返回 HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return "", false, errors.New("联通云盘秒传回执无效")
	}
	var v struct {
		Meta struct {
			Code string `json:"code"`
		} `json:"meta"`
		Result struct {
			HasFile json.RawMessage `json:"hasFile"`
			FID     string          `json:"fid"`
			FileID  string          `json:"fileId"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Meta.Code == "" || len(v.Result.HasFile) == 0 {
		return "", false, errors.New("联通云盘秒传回执无效")
	}
	if v.Meta.Code != "0000" {
		return "", false, &APIError{Code: v.Meta.Code, Desc: "秒传请求未成功"}
	}
	var flag any
	_ = json.Unmarshal(v.Result.HasFile, &flag)
	if toInt(flag) == 0 && (string(v.Result.HasFile) == "0" || string(v.Result.HasFile) == `"0"`) {
		return "", false, nil
	}
	if toInt(flag) != 1 {
		return "", false, errors.New("联通云盘秒传回执无效")
	}
	fid := v.Result.FID
	if fid == "" {
		fid = v.Result.FileID
	}
	if fid == "" {
		return "", false, errors.New("联通云盘秒传已命中，未返回文件标识，请检查远端")
	}
	return fid, true, nil
}

// UploadInfo prepares the encrypted metadata. File bytes are streamed by the
// host, not opened, buffered, base64-encoded or uploaded inside WASM.
func (c *Client) UploadInfo(name, parent, batch string, size int64) (string, error) {
	token, _ := c.Tokens()
	b, _ := json.Marshal(map[string]any{"spaceType": SpacePersonal, "directoryId": parent, "batchNo": batch, "fileName": name, "fileSize": size, "fileType": fileTypeOf(name), "deviceNo": c.DeviceNo(), "clientType": 1})
	return aesEncrypt(string(b), token)
}
