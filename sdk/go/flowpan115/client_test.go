package flowpan115

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUsageSendsExternalStorageRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"driver":"open","result":{"total":3000,"used":5,"available":2995,"user_id":42,"user_name":"tester"}}}`))
	}))
	defer server.Close()

	client := New(server.URL+"/", "secret")
	res, err := client.Usage(context.Background(), DriverAuto)
	if err != nil {
		t.Fatalf("Usage returned error: %v", err)
	}
	if gotPath != "/api/v1/storage/115/auto/usage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody == nil {
		t.Fatal("expected JSON body")
	}
	if res.Driver != DriverOpen {
		t.Fatalf("driver = %q", res.Driver)
	}
	if res.Result.Total != 3000 || res.Result.Available != 2995 || res.Result.UserName != "tester" {
		t.Fatalf("result = %+v", res.Result)
	}
}

func TestListPagePayloadAndTypedResult(t *testing.T) {
	showDir := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/storage/115/cookie/dir/list-page" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var payload ListPageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.CID != 123 || payload.Limit != 50 || payload.ShowDir == nil || !*payload.ShowDir {
			t.Fatalf("payload = %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"driver":"cookie","result":{"items":[{"file_id":9,"name":"movie.mkv","type":"file","pickcode":"abc"}],"total":1,"offset":0,"limit":50}}}`))
	}))
	defer server.Close()

	client := New(server.URL, "secret")
	res, err := client.ListPage(context.Background(), DriverCookie, ListPageRequest{CID: 123, Limit: 50, ShowDir: &showDir})
	if err != nil {
		t.Fatalf("ListPage returned error: %v", err)
	}
	if res.Driver != DriverCookie {
		t.Fatalf("driver = %q", res.Driver)
	}
	if len(res.Result.Items) != 1 || res.Result.Items[0].FileID != 9 || res.Result.Items[0].Pickcode != "abc" {
		t.Fatalf("items = %+v", res.Result.Items)
	}
}

func TestAPIErrorOnEnvelopeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":403,"msg":"external api disabled","data":{}}`))
	}))
	defer server.Close()

	client := New(server.URL, "secret")
	_, err := client.Usage(context.Background(), DriverAuto)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 403 || !strings.Contains(apiErr.Message, "disabled") {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}

func TestDefaultDriverCanSwitchBetweenOpenAndCookie(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"result":{"total":1,"available":1,"used":0}}}`))
	}))
	defer server.Close()

	client := New(server.URL, "secret")
	client.SetDefaultDriver(DriverOpen)
	if _, err := client.Usage(context.Background(), ""); err != nil {
		t.Fatalf("open default request failed: %v", err)
	}

	cookieClient := client.WithDefaultDriver(DriverCookie)
	if _, err := cookieClient.Usage(context.Background(), ""); err != nil {
		t.Fatalf("cookie default request failed: %v", err)
	}

	want := []string{
		"/api/v1/storage/115/open/usage",
		"/api/v1/storage/115/cookie/usage",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestCallRejectsMissingToken(t *testing.T) {
	client := New("http://127.0.0.1:8088", "")
	_, err := client.Usage(context.Background(), DriverAuto)
	if err == nil || !strings.Contains(err.Error(), "token is empty") {
		t.Fatalf("err = %v", err)
	}
}
