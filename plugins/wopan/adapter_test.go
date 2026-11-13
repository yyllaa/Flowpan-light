package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
)

func TestWopanQRCodeOnlyContract(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version    string `json:"version"`
		Definition struct {
			Version string                      `json:"version"`
			Network *pluginapi.NetworkPolicy    `json:"network"`
			Storage pluginapi.StorageDefinition `json:"storage"`
		} `json:"definition"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	s := manifest.Definition.Storage
	if manifest.Version == "" || manifest.Definition.Version != manifest.Version || manifest.Definition.Network == nil || manifest.Definition.Network.Proxy != pluginapi.ProxyDirect || len(s.LoginMethods) != 1 || s.LoginMethods[0] != "qrcode" || len(s.Fields) != 0 {
		t.Fatalf("QR-only package contract changed: %+v", manifest)
	}
}

func TestWopanRejectsManualLogin(t *testing.T) {
	for _, method := range []string{"token", "credentials", "cookie", ""} {
		call := &pluginapi.StorageCall{Account: "fixture", Operation: "login.begin", Params: pluginapi.StorageParams{Method: method}}
		result, err := execute(context.Background(), call, nil)
		if err != nil || !result.Failed || result.Message != "联通云盘登录方式无效" {
			t.Fatalf("manual login accepted %q: %+v %v", method, result, err)
		}
	}
	a, err := newAdapter(&pluginapi.StorageCall{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.ready(context.Background()); err == nil || err.Error() != "请扫码登录联通云盘" {
		t.Fatal("missing credential message", err)
	}
}

func TestOpaqueDirectoryRoundTrip(t *testing.T) {
	p := objectRef{ID: "0", Dir: true}
	for i := 0; i < 20; i++ {
		e := entry(wopan.Entry{ID: string(rune('a'+i)) + "+/=", Name: "目录", IsDir: true}, p)
		if e.ParentID != encodeRef(p) {
			t.Fatal("parent reference changed", i)
		}
		var err error
		p, err = decodeRef(e.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"", "0", "/", "wo1:not-base64", "wo1:e30"} {
		if _, err := decodeRef(s); err == nil {
			t.Fatal("invalid reference accepted", s)
		}
	}
}

func TestRotatedTokensSurviveBlankSettingsAndOldSeed(t *testing.T) {
	c := &pluginapi.StorageCall{Account: "fixture", Operation: "configure", Settings: map[string]any{"access_token": "1111111111111111-old", "refresh_token": "old-refresh"}}
	oldSeed := seed(c)
	c.Credentials, _ = json.Marshal(credentials{Access: "2222222222222222-new", Refresh: "new-refresh", UID: "stable-user", Seed: oldSeed})
	a, err := newAdapter(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token, _ := a.client.Tokens(); token != "2222222222222222-new" {
		t.Fatal("saved settings reverted the rotated token")
	}
	c.Settings = map[string]any{}
	a, err = newAdapter(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token, _ := a.client.Tokens(); token != "2222222222222222-new" {
		t.Fatal("blank settings discarded credentials")
	}
}

func TestWritePacingHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitWrite(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
