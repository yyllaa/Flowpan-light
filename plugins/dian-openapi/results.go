package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type resource struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Size    string   `json:"size,omitempty"`
	Points  int      `json:"points"`
	Owned   bool     `json:"owned"`
	PanType string   `json:"pan_type"`
	Files   []string `json:"files,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

func accountKey(email string) string {
	s := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(s[:])
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func re0MaskUser(s string) string {
	if len(s) < 3 {
		return "***"
	}
	return string([]rune(s)[:1]) + "***"
}
func re0Step(_ string, _ ...any) {} // Upstream login helper; progress is reported by the enclosing job.
func is115ShareLink(s string) bool {
	u, e := url.Parse(strings.TrimSpace(s))
	if e != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return (h == "115.com" || h == "115cdn.com" || h == "anxia.com" || strings.HasSuffix(h, ".115.com") || strings.HasSuffix(h, ".115cdn.com")) && strings.HasPrefix(u.Path, "/s/")
}

// Keep the complete protocol response within the host's 1 MiB limit even
// when a provider returns many large file lists. Titles and resource IDs stay
// available; file previews share a bounded byte budget.
func resourceResult(shares []dianShare) map[string]any {
	items := []resource{}
	remaining := 256 << 10
	for _, s := range shares {
		cost := 0
		if s.UnlockCost != nil {
			cost = *s.UnlockCost
		}
		pan := "115"
		if s.offline() {
			pan = "离线"
		}
		files := []string{}
		for _, file := range s.FileList {
			file = truncateStr(file, 512)
			if len(file) > remaining || len(files) >= 200 {
				break
			}
			remaining -= len(file)
			files = append(files, file)
		}
		items = append(items, resource{ID: dianRef(s), Title: truncateStr(s.title(), 512), Size: truncateStr(s.TotalSizeHuman, 64), Points: cost, Owned: s.owned(), PanType: pan, Files: files})
		if len(items) >= 200 {
			break
		}
	}
	for {
		data := map[string]any{"message": fmt.Sprintf("找到 %d 个资源", len(items)), "resources": items}
		b, _ := json.Marshal(data)
		if len(b) <= 768<<10 {
			return data
		}
		trimmed := false
		for n := len(items) - 1; n >= 0; n-- {
			if len(items[n].Files) > 0 {
				items[n].Files = nil
				trimmed = true
				break
			}
		}
		if !trimmed {
			items = items[:len(items)-1]
		}
	}
}
