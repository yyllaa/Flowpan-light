package wopan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestShareProtocolAndSingleSave(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope struct {
			Header struct {
				Key string `json:"key"`
			} `json:"header"`
			Body struct {
				Param string `json:"param"`
				Type  string `json:"secretType"`
			} `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&envelope)
		key := shareClientSecret
		if envelope.Header.Key == "ReceiveShare" {
			key = "abcdefghijklmnop"
			writes++
			if envelope.Body.Type != "TokenSecret" {
				t.Error("missing token secret")
			}
		}
		plain, err := aesDecrypt(envelope.Body.Param, key)
		if err != nil {
			t.Error(err)
		}
		var p map[string]any
		if json.Unmarshal(plain, &p) != nil {
			t.Error("invalid encrypted parameters")
		}
		if envelope.Header.Key == "ShareFileDetail" {
			if p["shareCode"] != "kpIX" {
				t.Error("share code lost")
			}
			data, _ := aesEncrypt(`[{"fileId":"f1","fileName":"sample.flac","fileType":"1"}]`, key)
			_ = json.NewEncoder(w).Encode(map[string]any{"RSP": map[string]any{"RSP_CODE": "0000", "DATA": data}})
		} else {
			if p["targetDirId"] != "folder-1" || len(p["fileList"].([]any)) != 1 {
				t.Error("wrong save target")
			}
			_, _ = w.Write([]byte(`{"RSP":{"RSP_CODE":"9999","RSP_DESC":"unknown receipt"}}`))
		}
	}))
	defer s.Close()
	c := New(Config{BaseURL: s.URL, HTTP: s.Client(), AccessToken: "abcdefghijklmnop"})
	share, err := c.ResolveShare(context.Background(), "https://panservice.mail.wo.cn/h5/wocloudshare/?shareId=95e4732b5f9e41fcbd096decdf84371b&shareCode=kpIX", "")
	if err != nil || len(share.Files) != 1 {
		t.Fatalf("resolve: %+v %v", share, err)
	}
	if err = c.ReceiveShare(context.Background(), share, "folder-1"); err == nil || writes != 1 {
		t.Fatalf("writes=%d err=%v", writes, err)
	}
}
func TestLiveShareReadOnly(t *testing.T) {
	raw := os.Getenv("FLOWPAN_WOPAN_SHARE_URL")
	if raw == "" {
		t.Skip("explicit public share URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c := New(Config{HTTP: &http.Client{Timeout: 30 * time.Second}})
	s, err := c.ResolveShare(ctx, raw, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("LIVE public share: files=%d name=%s; cloud writes=0", len(s.Files), s.Files[0].Name)
}
