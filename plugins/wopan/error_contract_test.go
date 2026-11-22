package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopanfixture"
)

type auditErrorTransport func(*http.Request) (*http.Response, error)

func (f auditErrorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAPIAuditWopanStructuredErrors(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		status     int
	}{
		{"auth", "", 401}, {"forbidden", "", 403}, {"rate-limit", "", 429}, {"temporary", "", 503}, {"business", "130027", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := &http.Client{Transport: auditErrorTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := wopanfixture.TokenA
				if tc.code != "" {
					body = fmt.Sprintf(`{"RSP":{"RSP_CODE":%q,"RSP_DESC":%q}}`, tc.code, body)
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			raw, _ := json.Marshal(credentials{Access: wopanfixture.TokenA, Refresh: "fixture-refresh", UID: "uid-a"})
			result, err := execute(context.Background(), &pluginapi.StorageCall{Account: "fixture", Operation: "list", Credentials: raw, Params: pluginapi.StorageParams{ParentID: rootID, Limit: 100}}, h)
			wire, _ := json.Marshal(result.Data["storage_error"])
			var failure pluginapi.StorageError
			_ = json.Unmarshal(wire, &failure)
			wantStatus := tc.status
			if wantStatus == 200 {
				wantStatus = 0
			}
			t.Logf("wopan fault=%s requests=%d code=%q status=%d structured=%t", tc.name, calls, failure.Code, failure.Status, failure.Validate() == nil)
			if err != nil || !result.Failed || calls != 1 || failure.Validate() != nil || failure.Code != tc.code || failure.Status != wantStatus || failure.Message != result.Message || strings.Contains(string(wire)+result.Message, wopanfixture.TokenA) {
				t.Fatalf("error contract lost: result=%+v wire=%s error=%v", result, wire, err)
			}
		})
	}
}
