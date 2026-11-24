// OpenList implements the storage protocol through the host's bounded network
// and private-state APIs. File bytes stay in the host streaming pipeline.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

var hostCall = defaultHostCall

// Literal tool dispatch keeps the release scanner's declared-call report exact.
func defaultHostCall(method string, input, out any) error {
	switch method {
	case "http.fetch":
		return pluginwasm.Call("http.fetch", input, out)
	case "storage.paths":
		return pluginwasm.Call("storage.paths", input, out)
	case "kv.get":
		return pluginwasm.Call("kv.get", input, out)
	case "kv.set":
		return pluginwasm.Call("kv.set", input, out)
	default:
		return fmt.Errorf("网盘宿主工具无效")
	}
}

type credentials struct {
	Username string `json:"username,omitempty"`
	Token    string `json:"token"`
	UID      int    `json:"uid"`
	BasePath string `json:"base_path"`
	Identity string `json:"identity"`
}
type adapter struct {
	c               *pluginapi.StorageCall
	endpoint        *url.URL
	root, namespace string
	creds           credentials
	rotated         bool
}
type object struct {
	Name     string            `json:"name"`
	Size     int64             `json:"size"`
	IsDir    bool              `json:"is_dir"`
	Modified time.Time         `json:"modified"`
	Sign     string            `json:"sign"`
	HashInfo map[string]string `json:"hash_info"`
}

func main() {
	pluginwasm.Serve(pluginwasm.Handlers{Storage: storage, Health: func(pluginwasm.Invocation) (pluginapi.Result, error) { return pluginapi.Result{Message: "ready"}, nil }})
}
func setting(c *pluginapi.StorageCall, k string) string {
	v, _ := c.Settings[k].(string)
	return strings.TrimSpace(v)
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func fail(code string, status int, message string) error {
	return &pluginapi.StorageError{Code: code, Status: status, Message: message}
}
func cleanPath(p string) (string, error) {
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") || path.Clean(p) != p || strings.ContainsAny(p, "\\\x00\r\n") {
		return "", fail("400", 400, "目录路径无效")
	}
	return p, nil
}
func validName(name string) bool {
	return strings.TrimSpace(name) != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00\r\n")
}
func storage(in pluginwasm.Invocation) (pluginapi.Result, error) {
	c := in.Request.Storage
	if c == nil {
		return pluginapi.Result{}, fmt.Errorf("缺少网盘调用")
	}
	u, err := pluginapi.StorageEndpoint(setting(c, "endpoint"))
	if err != nil {
		return pluginapi.Result{}, err
	}
	root, err := cleanPath(setting(c, "root"))
	if err != nil {
		return pluginapi.Result{}, err
	}
	a := &adapter{c: c, endpoint: u, root: root}
	if len(c.Credentials) > 0 {
		if err = json.Unmarshal(c.Credentials, &a.creds); err != nil {
			return pluginapi.Result{}, fmt.Errorf("登录凭据无效")
		}
	}
	a.namespace = hash([]any{u.String(), a.creds.UID, a.creds.BasePath, root})
	v, err := a.dispatch()
	r := pluginwasm.StorageResult(v)
	if a.rotated && c.Operation != "configure" && c.Operation != "verify" && c.Operation != "refresh" && !strings.HasPrefix(c.Operation, "login.") {
		b, _ := json.Marshal(a.creds)
		r.Data["storage_credentials"] = pluginapi.StorageCredentials{Credentials: b}
	}
	if err != nil {
		message := err.Error()
		for _, secret := range []string{a.creds.Token, setting(c, "token"), setting(c, "password"), setting(c, "otp_code"), setting(c, "directory_password")} {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[已隐藏]")
			}
		}
		r.Failed = true
		r.Message = message
		var detail *pluginapi.StorageError
		if errors.As(err, &detail) {
			copy := *detail
			copy.Message = message
			r.Data["storage_error"] = copy
		}
	}
	return r, nil
}
func (a *adapter) url(p string) string {
	u := *a.endpoint
	route, _ := url.Parse(p)
	u.Path += route.Path
	u.RawPath = ""
	u.RawQuery = route.RawQuery
	return u.String()
}
func (a *adapter) request(method, p string, body any, out any) error {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	var response pluginapi.StorageHTTPResponse
	err := hostCall("http.fetch", map[string]any{"url": a.url(p), "method": method, "headers": map[string]string{"Authorization": a.creds.Token, "Content-Type": "application/json"}, "body": string(raw)}, &response)
	if err != nil {
		return err
	}
	return decodeResponse(response, out)
}
func decodeResponse(response pluginapi.StorageHTTPResponse, out any) error {
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(response.Body), &envelope); err != nil {
		return fail("invalid_response", response.Status, "OpenList 返回内容格式无效")
	}
	if response.Status < 200 || response.Status >= 300 || envelope.Code != 200 {
		status := response.Status
		if status >= 200 && status < 300 && envelope.Code >= 400 && envelope.Code <= 599 {
			status = envelope.Code
		}
		if status >= 200 && status < 300 {
			status = 0
		}
		return fail(strconv.Itoa(envelope.Code), status, "OpenList："+envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
func (a *adapter) api(method, p string, body any, out any) error {
	err := a.request(method, p, body, out)
	var detail *pluginapi.StorageError
	if errors.As(err, &detail) && detail.Status == 401 && setting(a.c, "auth_mode") != "token" && setting(a.c, "password") != "" {
		old := a.creds.Identity
		if e := a.login(); e != nil {
			return e
		}
		if e := a.identify(); e != nil {
			return e
		}
		if old == "" || old != a.creds.Identity {
			return fail("account_changed", 401, "OpenList 账号已变化，请重新登录")
		}
		a.rotated = true
		return a.request(method, p, body, out)
	}
	return err
}
func (a *adapter) login() error {
	if setting(a.c, "auth_mode") == "token" {
		a.creds.Token = setting(a.c, "token")
		if a.creds.Token == "" {
			return fail("401", 401, "请填写 OpenList Token")
		}
		return nil
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := a.request("POST", "/api/auth/login", map[string]string{"username": setting(a.c, "username"), "password": setting(a.c, "password"), "otp_code": setting(a.c, "otp_code")}, &result); err != nil {
		return err
	}
	if result.Token == "" {
		return fail("401", 401, "OpenList 登录未返回 Token")
	}
	a.creds.Token = result.Token
	return nil
}
func (a *adapter) identify() error {
	var me struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		BasePath string `json:"base_path"`
		Disabled bool   `json:"disabled"`
		Role     int    `json:"role"`
	}
	if err := a.request("GET", "/api/me", nil, &me); err != nil {
		return err
	}
	if me.ID <= 0 || me.Disabled || me.Role == 1 {
		return fail("401", 401, "OpenList 账号未登录或已停用")
	}
	a.creds.UID = me.ID
	a.creds.Username = me.Username
	a.creds.BasePath = path.Clean("/" + strings.TrimPrefix(me.BasePath, "/"))
	a.creds.Identity = hash([]any{a.endpoint.String(), me.ID, a.creds.BasePath, a.root})
	a.namespace = hash([]any{a.endpoint.String(), me.ID, a.creds.BasePath, a.root})
	return nil
}
func (a *adapter) paths(op string, paths []string, id, p string) ([]pluginapi.StoragePathRef, error) {
	var refs []pluginapi.StoragePathRef
	err := hostCall("storage.paths", pluginapi.StoragePathsRequest{Operation: op, Namespace: a.namespace, Paths: paths, ID: id, Path: p}, &refs)
	return refs, err
}
func (a *adapter) resolve(id string) (string, error) {
	r, err := a.paths("resolve", nil, id, "")
	if err != nil {
		return "", err
	}
	if len(r) != 1 || (r[0].Path != a.root && !strings.HasPrefix(r[0].Path, strings.TrimSuffix(a.root, "/")+"/")) {
		return "", fail("404", 404, "文件引用不存在")
	}
	return r[0].Path, nil
}
func (a *adapter) bind(paths ...string) ([]pluginapi.StoragePathRef, error) {
	return a.paths("bind", paths, "", "")
}
func (a *adapter) auth() (pluginapi.StorageAuth, error) {
	old := a.creds.Identity
	if a.c.Operation == "configure" || a.c.Operation == "login.begin" || a.creds.Token == "" || a.c.Operation == "refresh" {
		if err := a.login(); err != nil {
			return pluginapi.StorageAuth{}, err
		}
	}
	if err := a.identify(); err != nil {
		return pluginapi.StorageAuth{}, err
	}
	if (a.c.Operation == "verify" || a.c.Operation == "refresh") && old != "" && old != a.creds.Identity {
		return pluginapi.StorageAuth{}, fail("account_changed", 401, "OpenList 账号或根目录已变化，请重新登录")
	}
	root, err := a.get(a.root)
	if err != nil {
		return pluginapi.StorageAuth{}, err
	}
	if !root.IsDir {
		return pluginapi.StorageAuth{}, fail("400", 400, "账号根目录必须是目录")
	}
	refs, err := a.bind(a.root)
	if err != nil {
		return pluginapi.StorageAuth{}, err
	}
	b, _ := json.Marshal(a.creds)
	return pluginapi.StorageAuth{State: "confirmed", Identity: a.creds.Identity, RootID: refs[0].ID, Name: a.creds.Username, Credentials: b}, nil
}
func (a *adapter) get(p string) (object, error) {
	var o object
	err := a.api("POST", "/api/fs/get", map[string]any{"path": p, "password": setting(a.c, "directory_password")}, &o)
	var detail *pluginapi.StorageError
	if errors.As(err, &detail) && (strings.Contains(strings.ToLower(detail.Message), "object not found") || strings.Contains(strings.ToLower(detail.Message), "no such file or directory")) {
		copy := *detail
		copy.Status = 404
		err = &copy
	}
	return o, err
}
func (a *adapter) entry(p string, o object) (pluginapi.StorageEntry, error) {
	parent := path.Dir(p)
	if p == a.root {
		parent = a.root
	}
	r, err := a.bind(p, parent)
	if err != nil {
		return pluginapi.StorageEntry{}, err
	}
	e := a.makeEntry(p, o, r[0].ID, r[1].ID)
	return e, nil
}
func (a *adapter) makeEntry(p string, o object, id, parent string) pluginapi.StorageEntry {
	display := "/" + strings.TrimPrefix(strings.TrimPrefix(p, a.root), "/")
	if p == a.root {
		display = "/"
	}
	modified := int64(0)
	if !o.Modified.IsZero() {
		modified = o.Modified.Unix()
	}
	e := pluginapi.StorageEntry{ID: id, ParentID: parent, Name: o.Name, Path: display, IsDir: o.IsDir, Size: o.Size, Modified: modified}
	for algorithm, value := range o.HashInfo {
		algorithm = strings.ToLower(strings.ReplaceAll(algorithm, "-", ""))
		if (algorithm == "sha1" || algorithm == "sha256" || algorithm == "md5") && value != "" {
			e.Hashes = append(e.Hashes, pluginapi.StorageHash{Algorithm: algorithm, Value: value})
		}
	}
	return e
}
func (a *adapter) list() (pluginapi.StoragePage, error) {
	p := a.c.Params
	parent, err := a.resolve(p.ParentID)
	if err != nil {
		return pluginapi.StoragePage{}, err
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := p.Offset
	if p.Cursor != "" {
		offset, err = strconv.Atoi(p.Cursor)
		if err != nil {
			return pluginapi.StoragePage{}, fail("400", 400, "分页游标无效")
		}
	}
	if offset < 0 {
		return pluginapi.StoragePage{}, fail("400", 400, "分页位置无效")
	}
	// Align arbitrary Flowpan offsets to OpenList's page contract. At most two
	// pages are needed; no complete-directory fetch or client-side sorting.
	start := offset % limit
	page := offset/limit + 1
	var result struct {
		Content []object `json:"content"`
		Total   int      `json:"total"`
	}
	load := func(page int) error {
		return a.api("POST", "/api/fs/list", map[string]any{"path": parent, "password": setting(a.c, "directory_password"), "page": page, "per_page": limit, "refresh": p.Refresh}, &result)
	}
	if err = load(page); err != nil {
		return pluginapi.StoragePage{}, err
	}
	total := result.Total
	items := []object{}
	if start < len(result.Content) {
		items = append(items, result.Content[start:]...)
	}
	if start > 0 && offset+len(items) < total && len(items) < limit {
		if err = load(page + 1); err != nil {
			return pluginapi.StoragePage{}, err
		}
		need := limit - len(items)
		if need > len(result.Content) {
			need = len(result.Content)
		}
		items = append(items, result.Content[:need]...)
	}
	paths := make([]string, len(items))
	for i, o := range items {
		if !validName(o.Name) {
			return pluginapi.StoragePage{}, fail("invalid_response", 0, "OpenList 返回文件名无效")
		}
		paths[i] = path.Join(parent, o.Name)
	}
	refs, err := a.bind(paths...)
	if err != nil {
		return pluginapi.StoragePage{}, err
	}
	out := pluginapi.StoragePage{Items: []pluginapi.StorageEntry{}, Total: total, Offset: offset, Limit: limit}
	for i, o := range items {
		out.Items = append(out.Items, a.makeEntry(paths[i], o, refs[i].ID, p.ParentID))
	}
	out.HasMore = offset+len(items) < total
	if out.HasMore {
		out.NextCursor = strconv.Itoa(offset + len(items))
		if len(items) == 0 {
			return out, fail("invalid_page", 0, "OpenList 分页未前进")
		}
	}
	return out, nil
}
func (a *adapter) link() (pluginapi.StorageLink, error) {
	p, err := a.resolve(a.c.Params.ID)
	if err != nil {
		return pluginapi.StorageLink{}, err
	}
	o, err := a.get(p)
	if err != nil {
		return pluginapi.StorageLink{}, err
	}
	if o.IsDir {
		return pluginapi.StorageLink{}, fail("400", 400, "目录不能播放")
	}
	u := *a.endpoint
	u.Path += "/d" + path.Join(a.creds.BasePath, p)
	u.RawPath = ""
	query := url.Values{"d": []string{"1"}}
	if o.Sign != "" {
		query.Set("sign", o.Sign)
	}
	u.RawQuery = query.Encode()
	// The download route honors OpenList's storage proxy policy; forcing /p
	// rejects otherwise valid direct downloads when that storage disallows proxying.
	// Credentials are never placed in playback URLs or client-visible headers.
	return pluginapi.StorageLink{URL: u.String(), ExpiresAt: time.Now().Add(2 * time.Minute)}, nil
}
func (a *adapter) dispatch() (any, error) {
	switch a.c.Operation {
	case "configure", "login.begin", "verify", "refresh":
		return a.auth()
	case "list":
		return a.list()
	case "stat":
		p, err := a.resolve(a.c.Params.ID)
		if err != nil {
			return nil, err
		}
		o, err := a.get(p)
		if err != nil {
			return nil, err
		}
		return a.entry(p, o)
	case "read", "link":
		return a.link()
	case "mkdir", "rename", "move", "copy", "delete":
		return a.mutate()
	case "upload":
		return a.upload()
	default:
		return nil, fmt.Errorf("OpenList 不支持此操作：%s", a.c.Operation)
	}
}
