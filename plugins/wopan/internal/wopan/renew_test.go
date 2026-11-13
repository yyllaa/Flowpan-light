package wopan

import (
	"context"
	"testing"
	"time"
)

// 并发续期去重：stale 与当前 token 不同时直接返回，不打网络
func TestRenewSkipsWhenAlreadyRenewed(t *testing.T) {
	c := New(Config{AccessToken: "NEW", RefreshToken: "R"})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.renew(ctx, "OLD"); err != nil {
		t.Fatalf("应直接返回 nil，实际: %v", err)
	}
}

// 没有 refresh_token 时必须是 ErrNeedLogin（上层据此回 401 而不是重试）
func TestRefreshWithoutToken(t *testing.T) {
	c := New(Config{AccessToken: "A"})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.Refresh(ctx); err != ErrNeedLogin {
		t.Fatalf("want ErrNeedLogin, got %v", err)
	}
}
