package wopan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// REST 通道未登录码是 10002404：应触发续期，没有 refresh_token 时返回 ErrNeedLogin
func TestRestNeedLoginCode(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"code":"10002404","message":"用户未登录"}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, AccessToken: "stale-token-0000000000000000000000000000"})
	_, err := c.Storage(context.Background())
	if !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("want ErrNeedLogin, got %v", err)
	}
	if hits == 0 {
		t.Fatal("没有真正发出请求")
	}
}

// dispatcher 通道未登录码 1001 同样要续期（这里验证 1001 会走续期而不是直接抛错）
func TestDispatcherNeedLoginCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"RSP":{"RSP_CODE":"1001","RSP_DESC":"登录失败"}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, AccessToken: "stale-token-0000000000000000000000000000"})
	_, err := c.List(context.Background(), SpacePersonal, "0", "")
	if !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("want ErrNeedLogin, got %v", err)
	}
}

func TestIsNeedLogin(t *testing.T) {
	for _, code := range []string{"1001", "10002404"} {
		if !IsNeedLogin(code) {
			t.Fatalf("%s 应判定为需要登录", code)
		}
	}
	if IsNeedLogin("0000") {
		t.Fatal("0000 不应判定为需要登录")
	}
}
