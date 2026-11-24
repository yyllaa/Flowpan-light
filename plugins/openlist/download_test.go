package main

import (
	"net/url"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func TestDownloadLinkHonorsStorageProxyPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, base, sign string
	}{
		{"unsigned_direct_storage", "/", ""},
		{"signed_storage", "/", "signature:1764000000"},
		{"restricted_user_root", "/用户 甲", "signature:1764000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureAdapter(t)
			a.creds.BasePath = tc.base
			objectPath := "/media/目录/欢太 工具#箱+%?.apk"
			calls := 0
			useHost(t, func(tool string, input, out any) error {
				switch tool {
				case "storage.paths":
					request := input.(pluginapi.StoragePathsRequest)
					if request.Operation != "resolve" || request.ID != "file-ref" {
						t.Fatal("unexpected path lookup", request)
					}
					return jsonInto([]pluginapi.StoragePathRef{{ID: "file-ref", Path: objectPath}}, out)
				case "http.fetch":
					calls++
					request := input.(map[string]any)
					u, _ := url.Parse(request["url"].(string))
					if u.Path != "/prefix/api/fs/get" {
						t.Fatal("unexpected network request", u.Path)
					}
					return jsonInto(pluginapi.StorageHTTPResponse{Status: 200, Body: `{"code":200,"data":{"name":"file.apk","size":4468785,"sign":"` + tc.sign + `"}}`}, out)
				default:
					t.Fatal("unexpected tool", tool)
					return nil
				}
			})
			a.c.Params.ID = "file-ref"
			link, err := a.link()
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(link.URL)
			if err != nil {
				t.Fatal(err)
			}
			expected := "/prefix/d" + strings.TrimSuffix(tc.base, "/") + objectPath
			if u.Path != expected || u.Query().Has("force") || u.Query().Get("sign") != tc.sign {
				t.Fatalf("download route ignores storage policy or loses path/sign: %s", link.URL)
			}
			if u.Fragment != "" || len(link.Headers) != 0 || strings.Contains(link.URL, a.creds.Token) || strings.Contains(link.URL, "fixture-password") || calls != 1 {
				t.Fatal("download link exposes credentials or adds unnecessary API calls", calls)
			}
		})
	}
}
