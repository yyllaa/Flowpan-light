package wopan

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// 服务端 9999/500/1000 要翻成能照着改的提示，其它错误码原样保留（保持 APIError 类型）
func TestExplainOp(t *testing.T) {
	for _, code := range []string{"9999", "500", "1000"} {
		err := explainOp("重命名", &APIError{Code: code, Desc: "x"})
		if !strings.Contains(err.Error(), "id 类型不匹配") {
			t.Fatalf("%s 未被翻译: %v", code, err)
		}
	}

	other := explainOp("重命名", &APIError{Code: "130027", Desc: "重名"})
	var ae *APIError
	if !errors.As(other, &ae) || ae.Code != "130027" {
		t.Fatalf("其它错误码不应被改写: %v", other)
	}

	if explainOp("重命名", nil) != nil {
		t.Fatal("nil 应原样返回 nil")
	}
}

// 空 fid 必须在发请求前就挡掉（目录没有 fid）
func TestDownloadURLEmptyFID(t *testing.T) {
	c := New(Config{})
	_, err := c.DownloadURL(context.Background(), "", "0")
	if err == nil || !strings.Contains(err.Error(), "fid 为空") {
		t.Fatalf("want fid 为空 错误, got %v", err)
	}
}
