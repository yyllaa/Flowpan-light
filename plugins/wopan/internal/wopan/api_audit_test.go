package wopan

import (
	"errors"
	"testing"
)

func TestAPIAuditWopanExplanationKeepsBusinessCode(t *testing.T) {
	for _, code := range []string{"9999", "500", "1000", "500010006", "130027"} {
		t.Run(code, func(t *testing.T) {
			err := explainOp("重命名", &APIError{Code: code, Desc: "fixture"})
			var business interface{ BusinessCode() string }
			t.Logf("wopan code=%s typed=%t", code, errors.As(err, &business))
			if !errors.As(err, &business) || business.BusinessCode() != code {
				t.Fatalf("business code lost: %v", err)
			}
		})
	}
}
