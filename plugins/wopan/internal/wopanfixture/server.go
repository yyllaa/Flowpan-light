// Package wopanfixture is a local protocol peer used only by regression tests.
// It validates wire encryption/signatures, not a JSON stand-in for the adapter.
package wopanfixture

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TokenA = "1111111111111111-fixture-a"
const TokenB = "2222222222222222-fixture-b"
const TokenNew = "3333333333333333-fixture-a"
const PNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j3ioAAAAASUVORK5CYII="
const secret = "s72g2VY46ahlK588"

type File struct {
	ID       string `json:"id"`
	FID      string `json:"fid"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Size     int64  `json:"size"`
	Version  int    `json:"fileVersion"`
	Modified string `json:"updateTime"`
}
type Server struct {
	Server     *httptest.Server
	mu         sync.Mutex
	Calls      map[string]int
	Files      map[string][]File
	Token      string
	Expire     bool
	QRState    int
	QRToken    string
	QRRefresh  string
	QuickCode  string
	QuickHit   bool
	FailFinal  bool
	Parts      int
	Bytes      int64
	SeenSHA    string
	Errors     []string
	BatchCode  string
	BatchReply json.RawMessage
}
type transport struct {
	base http.RoundTripper
	url  string
}

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	u := *r.URL
	copy.URL = &u
	target := strings.TrimPrefix(t.url, "https://")
	copy.Host = r.URL.Host
	copy.URL.Host = target
	return t.base.RoundTrip(copy)
}
func (s *Server) Client() *http.Client {
	c := s.Server.Client()
	c.Transport = transport{c.Transport, s.Server.URL}
	return c
}
func New() *Server {
	s := &Server{Calls: map[string]int{}, Files: map[string][]File{}, Token: TokenA, QRState: 1, QRToken: TokenA, QRRefresh: "refresh-a", QuickCode: "0000"}
	for _, owner := range []string{"a", "b"} {
		s.Files[owner+":0"] = []File{{ID: "dir-" + owner, Name: "影视", Type: 0}, {ID: "media-" + owner, FID: "opaque+/=" + owner, Name: "测试影片.mkv", Type: 1, Size: 16, Version: 1, Modified: "20261111120000"}}
		s.Files[owner+":dir-"+owner] = []File{{ID: "nested-" + owner, FID: "nested+/=" + owner, Name: "01.测试.m4a", Type: 1, Size: 16, Version: 1}}
	}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	return s
}
func (s *Server) Close()               { s.Server.Close() }
func (s *Server) Count(key string) int { s.mu.Lock(); defer s.mu.Unlock(); return s.Calls[key] }
func (s *Server) Check() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Errors) > 0 {
		return fmt.Errorf("protocol errors: %v", s.Errors)
	}
	return nil
}
func (s *Server) SetExpired()          { s.mu.Lock(); s.Expire = true; s.mu.Unlock() }
func (s *Server) SetQRState(state int) { s.mu.Lock(); s.QRState = state; s.mu.Unlock() }
func (s *Server) SetQRAccount(token, refresh string) {
	s.mu.Lock()
	s.QRToken, s.QRRefresh = token, refresh
	s.mu.Unlock()
}
func (s *Server) SetQuick(code string, hit bool) {
	s.mu.Lock()
	s.QuickCode, s.QuickHit = code, hit
	s.mu.Unlock()
}
func (s *Server) SetFailFinal()        { s.mu.Lock(); s.FailFinal = true; s.mu.Unlock() }
func (s *Server) Counts() (int, int64) { s.mu.Lock(); defer s.mu.Unlock(); return s.Parts, s.Bytes }
func (s *Server) Fill(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files["a:0"] = nil
	for i := 0; i < n; i++ {
		s.Files["a:0"] = append(s.Files["a:0"], File{ID: fmt.Sprint(i + 1), FID: fmt.Sprint(i+1) + "+/=", Name: fmt.Sprintf("%04d.m4a", i), Type: 1, Size: 16})
	}
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func rest(w http.ResponseWriter, result any) {
	jsonOut(w, map[string]any{"meta": map[string]string{"code": "200"}, "result": result})
}
func rsp(w http.ResponseWriter, code string, result any) {
	jsonOut(w, map[string]any{"RSP": map[string]any{"RSP_CODE": code, "RSP_DESC": "fixture", "DATA": result}})
}
func decrypt(enc, key string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(raw) == 0 || len(raw)%16 != 0 {
		return nil, fmt.Errorf("invalid encrypted body")
	}
	if len(key) < 16 {
		return nil, fmt.Errorf("invalid key")
	}
	block, _ := aes.NewCipher([]byte(key[:16]))
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, []byte("wNSOYIB1k1DjY5lA")).CryptBlocks(plain, raw)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > 16 {
		return nil, fmt.Errorf("invalid padding")
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return plain[:len(plain)-padding], nil
}
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls[r.URL.Path]++
	fail := func(message string) { s.Errors = append(s.Errors, message); http.Error(w, message, 400) }
	if r.URL.Path == "/download" {
		if r.Header.Get("Access-Token") != "" || r.Header.Get("X-YP-Access-Token") != "" {
			fail("credential leaked to CDN")
			return
		}
		http.ServeContent(w, r, "video.mkv", time.Unix(1, 0), bytes.NewReader([]byte("0123456789abcdef")))
		return
	}
	if !strings.Contains(r.UserAgent(), "wocloud/2.8.13") {
		fail("wrong User-Agent")
		return
	}
	if r.URL.Path == "/wohome/open/v1/QRCode/generate" {
		rest(w, map[string]string{"uuid": "uuid+/=&test", "image": strings.TrimPrefix(PNG, "data:image/png;base64,")})
		return
	}
	if r.URL.Path == "/wohome/open/v1/QRCode/query" {
		if r.URL.Query().Get("uuid") != "uuid+/=&test" {
			fail("UUID query changed")
			return
		}
		rest(w, map[string]any{"state": s.QRState, "token": s.QRToken, "refreshToken": s.QRRefresh})
		return
	}
	token := r.Header.Get("Access-Token")
	if token == "" {
		token = r.Header.Get("access-token")
	}
	owner := "a"
	if token == TokenB {
		owner = "b"
	}
	if r.URL.Path == "/api/bff/member/info/card/storage" {
		rest(w, map[string]int64{"usedSpace": 64, "allSpace": 1 << 40})
		return
	}
	if r.URL.Path == "/openapi/transfer/quickTransfer" {
		var b struct {
			SHA    string `json:"sha256"`
			Size   int64  `json:"fileSize"`
			Name   string `json:"fileName"`
			Parent string `json:"directoryId"`
		}
		if json.NewDecoder(r.Body).Decode(&b) != nil || len(b.SHA) != 64 {
			fail("quick body")
			return
		}
		s.SeenSHA = b.SHA
		hit := 0
		fid := ""
		if s.QuickHit && s.QuickCode == "0000" {
			hit = 1
			fid = "quick-fid"
			s.Files[owner+":"+b.Parent] = append(s.Files[owner+":"+b.Parent], File{ID: "quick-id", FID: fid, Name: b.Name, Type: 1, Size: b.Size})
		}
		jsonOut(w, map[string]any{"meta": map[string]string{"code": s.QuickCode}, "result": map[string]any{"hasFile": hit, "fid": fid}})
		return
	}
	if r.URL.Path == "/openapi/client/upload2C" {
		mr, err := r.MultipartReader()
		if err != nil {
			fail("multipart missing")
			return
		}
		fields := map[string]string{}
		var count int64
		for {
			part, e := mr.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				fail("multipart read")
				return
			}
			if part.FormName() == "file" {
				count, _ = io.Copy(io.Discard, part)
			} else {
				b, _ := io.ReadAll(io.LimitReader(part, 65537))
				fields[part.FormName()] = string(b)
			}
		}
		part, _ := strconv.Atoi(fields["partIndex"])
		total, _ := strconv.Atoi(fields["totalPart"])
		expected, _ := strconv.ParseInt(fields["partSize"], 10, 64)
		if fields["accessToken"] != token || fields["channel"] != "wocloud" || count != expected || part < 1 || total < part || r.ContentLength <= count {
			fail("upload fields or streaming length")
			return
		}
		plain, e := decrypt(fields["fileInfo"], token)
		var info struct {
			Name   string `json:"fileName"`
			Size   int64  `json:"fileSize"`
			Parent string `json:"directoryId"`
		}
		if e != nil || json.Unmarshal(plain, &info) != nil || info.Name != fields["fileName"] {
			fail("fileInfo encryption")
			return
		}
		s.Parts++
		s.Bytes += count
		fid := ""
		if part == total {
			fid = "uploaded-fid"
			s.Files[owner+":"+info.Parent] = append(s.Files[owner+":"+info.Parent], File{ID: "uploaded-id", FID: fid, Name: info.Name, Type: 1, Size: info.Size})
			if s.FailFinal {
				s.FailFinal = false
				w.WriteHeader(502)
				return
			}
		}
		jsonOut(w, map[string]any{"code": "0000", "data": map[string]string{"fid": fid}})
		return
	}
	if r.URL.Path == "/wohome/pass/file/batch/createTask" {
		s.batch(w, r, owner)
		return
	}
	if r.URL.Path != "/api-user/dispatcher" && r.URL.Path != "/wohome/dispatcher" {
		fail("unexpected path " + r.URL.Path)
		return
	}
	var body struct {
		Header struct {
			Key     string `json:"key"`
			Time    string `json:"resTime"`
			Seq     string `json:"reqSeq"`
			Channel string `json:"channel"`
			Sign    string `json:"sign"`
		} `json:"header"`
		Body struct {
			Param string `json:"param"`
		} `json:"body"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		fail("dispatcher body")
		return
	}
	h := body.Header
	sum := md5.Sum([]byte(h.Key + h.Time + h.Seq + h.Channel))
	if h.Sign != hex.EncodeToString(sum[:]) {
		fail("dispatcher signature")
		return
	}
	s.Calls[h.Key]++
	key := token
	if h.Key == "ShareFileDetail" {
		key = "XFmi9GS2hzk98jGX"
	}
	if r.URL.Path == "/api-user/dispatcher" {
		key = secret
	}
	if key == "" {
		key = secret
	}
	data, err := decrypt(body.Body.Param, key)
	var p map[string]any
	if err != nil || json.Unmarshal(data, &p) != nil {
		fail("dispatcher encryption")
		return
	}
	if h.Key == "ShareFileDetail" {
		if h.Channel != "100002" || p["shareCode"] != "kpIX" {
			fail("share public parameters")
			return
		}
		rsp(w, "0000", []map[string]any{{"id": "share-file", "fileName": "sample.flac", "type": "1", "fileType": "3"}})
		return
	}
	if h.Key == "ReceiveShare" {
		if h.Channel != "100002" || p["clientId"] != "1001000035" || p["targetDirId"] != "0" || len(p["fileList"].([]any)) != 1 {
			fail("share receive parameters")
			return
		}
		s.Files[owner+":0"] = append(s.Files[owner+":0"], File{ID: "saved-share-" + owner, Name: "sample.flac", Type: 1, Size: 64})
		rsp(w, "0000", map[string]any{})
		return
	}
	if p["clientId"] != "1001000209" {
		fail("client ID")
		return
	}
	if h.Key == "AppRefreshToken" {
		s.Token = TokenNew
		s.Expire = false
		rsp(w, "0000", map[string]any{"access_token": TokenNew, "refresh_token": "refresh-new-a", "expires_in": 7200})
		return
	}
	if s.Expire && token != TokenB {
		rsp(w, "1001", nil)
		return
	}
	switch h.Key {
	case "AppQueryUser":
		if p["accessToken"] != token || (token != s.Token && token != TokenB) {
			rsp(w, "1001", nil)
			return
		}
		rsp(w, "0000", map[string]string{"userId": "uid-" + owner, "userName": "联通账号 " + owner})
	case "QueryAllFiles":
		parent, _ := p["parentDirectoryId"].(string)
		page, _ := p["pageNum"].(float64)
		size, _ := p["pageSize"].(float64)
		items := s.Files[owner+":"+parent]
		start := int(page * size)
		end := start + int(size)
		if start > len(items) {
			start = len(items)
		}
		if end > len(items) {
			end = len(items)
		}
		rsp(w, "0000", map[string]any{"files": items[start:end], "hasNext": end < len(items)})
	case "GetDownloadUrlV2":
		fids, _ := p["fidList"].([]any)
		if len(fids) != 1 || !strings.Contains(fmt.Sprint(fids[0]), "+/=") {
			fail("FID changed")
			return
		}
		rsp(w, "0000", map[string]any{"list": []map[string]string{{"downloadUrl": "https://tjdownload.pan.wo.cn/download?sig=fake%2B%2F%3D"}}})
	case "CreateDirectory":
		parent, _ := p["parentDirectoryId"].(string)
		name, _ := p["directoryName"].(string)
		id := "new-dir"
		s.Files[owner+":"+parent] = append(s.Files[owner+":"+parent], File{ID: id, Name: name, Type: 0})
		rsp(w, "0000", map[string]string{"id": id})
	case "RenameFileOrDirectory":
		id, _ := p["id"].(string)
		name, _ := p["name"].(string)
		for k, files := range s.Files {
			if strings.HasPrefix(k, owner+":") {
				for i := range files {
					if files[i].ID == id {
						files[i].Name = name
						files[i].Version++
					}
				}
			}
		}
		rsp(w, "0000", 2)
	default:
		fail("unexpected dispatcher " + h.Key)
	}
}
