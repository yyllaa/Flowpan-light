package wopan

import (
	"context"
	"testing"
	"time"
)

// 写操作节流：第二次调用必须等够间隔；ctx 取消要立刻返回
func TestThrottleWrite(t *testing.T) {
	c := New(Config{MinWriteGap: 100 * time.Millisecond})
	start := time.Now()
	if err := c.throttleWrite(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.throttleWrite(context.Background()); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el < 90*time.Millisecond {
		t.Fatalf("第二次写没被节流，耗时 %v", el)
	}

	// 负值 = 不限速
	off := New(Config{MinWriteGap: -1})
	start = time.Now()
	_ = off.throttleWrite(context.Background())
	_ = off.throttleWrite(context.Background())
	if el := time.Since(start); el > 20*time.Millisecond {
		t.Fatalf("关闭节流后不应等待，耗时 %v", el)
	}

	// 等待中取消
	slow := New(Config{MinWriteGap: 5 * time.Second})
	_ = slow.throttleWrite(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := slow.throttleWrite(ctx); err == nil {
		t.Fatal("期望 ctx 取消错误")
	}
}
