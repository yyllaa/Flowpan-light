package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func fixtureAdapter(t *testing.T) *adapter {
	t.Helper()
	u, _ := url.Parse("http://service:5244/prefix")
	a := &adapter{c: &pluginapi.StorageCall{Account: "account-a", Settings: map[string]any{"endpoint": u.String(), "auth_mode": "password", "username": "admin", "password": "fixture-password"}}, endpoint: u, root: "/media", creds: credentials{Token: "old-token", UID: 2, BasePath: "/"}}
	a.creds.Identity = hash([]any{u.String(), 2, "/", "/media"})
	a.namespace = a.creds.Identity
	return a
}
func jsonInto(v, out any) error {
	if out == nil {
		return nil
	}
	raw, _ := json.Marshal(v)
	return json.Unmarshal(raw, out)
}
func useHost(t *testing.T, f func(string, any, any) error) {
	t.Helper()
	old := hostCall
	hostCall = f
	t.Cleanup(func() { hostCall = old })
}

func TestExpiredTokenReauthOnceAndCheckIdentity(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same_user", true: "changed_user"}[changed], func(t *testing.T) {
			a := fixtureAdapter(t)
			requests := []string{}
			reads := 0
			useHost(t, func(tool string, input, out any) error {
				p := input.(map[string]any)
				u, _ := url.Parse(p["url"].(string))
				requests = append(requests, u.Path)
				data := any(nil)
				code := 200
				message := "success"
				switch u.Path {
				case "/prefix/api/fs/get":
					reads++
					if reads == 1 {
						code = 401
						message = "Token is expired"
					} else {
						data = map[string]any{"name": "film.mp4", "size": 7}
					}
				case "/prefix/api/auth/login":
					data = map[string]any{"token": "new-token"}
				case "/prefix/api/me":
					id := 2
					if changed {
						id = 3
					}
					data = map[string]any{"id": id, "role": 0, "base_path": "/"}
				default:
					t.Fatal("unexpected request", u.Path)
				}
				b, _ := json.Marshal(map[string]any{"code": code, "message": message, "data": data})
				return jsonInto(pluginapi.StorageHTTPResponse{Status: 200, Body: string(b)}, out)
			})
			_, err := a.get("/media/film.mp4")
			if changed {
				if err == nil || reads != 1 {
					t.Fatal("new user reused original reference", err, reads)
				}
			} else if err != nil || reads != 2 || !a.rotated || a.creds.Token != "new-token" {
				t.Fatal("refresh", err, requests)
			}
		})
	}
}

func TestPermissionAndQuotaErrorsDoNotRetry(t *testing.T) {
	for _, status := range []int{403, 429, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			a := fixtureAdapter(t)
			calls := 0
			useHost(t, func(_ string, _ any, out any) error {
				calls++
				raw, _ := json.Marshal(map[string]any{"code": status, "message": "fixture failure"})
				return jsonInto(pluginapi.StorageHTTPResponse{Status: 200, Body: string(raw)}, out)
			})
			_, err := a.get("/media/file")
			var e *pluginapi.StorageError
			if !errors.As(err, &e) || e.Status != status || calls != 1 {
				t.Fatal("error type/status or retries", err, calls)
			}
		})
	}
}

func TestTaskTerminalStateAndResumeUseSavedID(t *testing.T) {
	for _, state := range []int{2, 4, 7} {
		t.Run(string(rune(state+'0')), func(t *testing.T) {
			a := fixtureAdapter(t)
			calls := 0
			useHost(t, func(_ string, input, out any) error {
				calls++
				p := input.(map[string]any)
				u, _ := url.Parse(p["url"].(string))
				if u.Path != "/prefix/api/task/copy/info" || u.Query().Get("tid") != "saved-task" || p["method"] != "POST" {
					t.Fatal("task route", u)
				}
				raw, _ := json.Marshal(map[string]any{"code": 200, "data": map[string]any{"id": "saved-task", "state": state}})
				return jsonInto(pluginapi.StorageHTTPResponse{Status: 200, Body: string(raw)}, out)
			})
			err := a.waitTasks("copy", []remoteTask{{ID: "saved-task"}})
			if (err == nil) != (state == 2) || calls != 1 {
				t.Fatal("terminal state", state, err, calls)
			}
		})
	}
}

func TestUploadUnknownReceiptNeverResends(t *testing.T) {
	a := fixtureAdapter(t)
	cp, _ := json.Marshal(uploadCheckpoint{Path: "/media/file.bin", Size: 5, SHA256: "fixture-sha", Namespace: a.namespace})
	a.c.Params = pluginapi.StorageParams{ParentID: "parent", Name: "file.bin", Upload: &pluginapi.StorageUpload{Size: 5, Checkpoint: cp, Resuming: true}}
	calls := 0
	useHost(t, func(tool string, input, out any) error {
		calls++
		if tool == "storage.paths" {
			return jsonInto([]pluginapi.StoragePathRef{{ID: "parent", Path: "/media"}}, out)
		}
		if tool != "http.fetch" {
			t.Fatal(tool)
		}
		p := input.(map[string]any)
		if strings.Contains(p["url"].(string), "/put") {
			t.Fatal("unknown upload replayed")
		}
		return jsonInto(pluginapi.StorageHTTPResponse{Status: 200, Body: `{"code":200,"data":{"name":"file.bin","size":5}}`}, out)
	})
	step, err := a.upload()
	var detail *pluginapi.StorageError
	if !errors.As(err, &detail) || detail.Code != "upload_receipt_unknown" || step.Request != nil || calls != 2 {
		t.Fatal("unknown receipt", step, err, calls)
	}
}

func TestDecodePreservesHTTPStatusAndRejectsBadReceipts(t *testing.T) {
	for _, r := range []pluginapi.StorageHTTPResponse{{Status: 200, Body: `{"code":403,"message":"denied"}`}, {Status: 429, Body: `{"code":429,"message":"quota"}`}, {Status: 502, Body: `<html>bad gateway</html>`}, {Status: 200, Body: `truncated`}} {
		var e *pluginapi.StorageError
		err := decodeResponse(r, nil)
		if !errors.As(err, &e) {
			t.Fatal("untyped failure", err)
		}
		if r.Status >= 400 && e.Status != r.Status {
			t.Fatal("HTTP status lost", e)
		}
	}
}
