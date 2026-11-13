package wopan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// 读接口偶发 9999 时要自动重试一次并成功
func TestDispatcherRetryOnceOn9999(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(`{"RSP":{"RSP_CODE":"9999","RSP_DESC":"系统异常"}}`))
			return
		}
		// 第二次：返回一个合法但空的加密 DATA（空 JSON 的 base64 由服务端给，这里给明文对象）
		_, _ = w.Write([]byte(`{"RSP":{"RSP_CODE":"0000","RSP_DESC":"ok","DATA":{"files":[]}}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, AccessToken: "0123456789abcdef0123456789abcdef"})
	got, err := c.List(context.Background(), SpacePersonal, "0", "")
	if err != nil {
		t.Fatalf("应重试后成功，实际错误: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("期望空列表，得到 %d 条", len(got))
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("期望请求 2 次（1 次 9999 + 1 次重试），实际 %d", calls)
	}
}

func TestDispatcherNeverReplaysAmbiguousWrite(t *testing.T) {
	for _, operation := range []string{"mkdir", "rename"} {
		for _, response := range []string{
			`{"RSP":{"RSP_CODE":"9999","RSP_DESC":"system error"}}`,
			`{"RSP":{"RSP_CODE":"500","RSP_DESC":"bad argument"}}`,
			`not JSON`,
		} {
			t.Run(operation+"/"+response, func(t *testing.T) {
				var calls int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(&calls, 1)
					_, _ = w.Write([]byte(response))
				}))
				defer srv.Close()
				c := New(Config{BaseURL: srv.URL, AccessToken: "0123456789abcdef0123456789abcdef"})
				var err error
				if operation == "mkdir" {
					_, err = c.Mkdir(context.Background(), SpacePersonal, "0", "test", "")
				} else {
					_, err = c.Rename(context.Background(), SpacePersonal, "fid", "test", false, "", "", "")
				}
				if err == nil || atomic.LoadInt32(&calls) != 1 {
					t.Fatalf("ambiguous write replayed: calls=%d err=%v", calls, err)
				}
			})
		}
	}
}
