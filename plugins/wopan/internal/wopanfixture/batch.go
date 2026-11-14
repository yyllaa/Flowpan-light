package wopanfixture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) SetBatchReply(code, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BatchCode, s.BatchReply = code, json.RawMessage(result)
}

// Called with s.mu held. Verify account, object types and version on the wire.
func (s *Server) batch(w http.ResponseWriter, r *http.Request, owner string) {
	fail := func(message string) { s.Errors = append(s.Errors, message); http.Error(w, message, 400) }
	var p struct {
		Task        string   `json:"taskType"`
		Space       string   `json:"sourceSpaceType"`
		TargetSpace string   `json:"targetSpaceType"`
		Files       []string `json:"sourceFileIds"`
		Dirs        []string `json:"sourceDirectoryIds"`
		Target      string   `json:"targetDirectoryId"`
		Version     string   `json:"fileVersion"`
		Device      string   `json:"deviceNo"`
	}
	if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&p) != nil || p.Space != "0" || p.Files == nil || p.Dirs == nil || p.Device == "" || len(p.Files)+len(p.Dirs) == 0 || (p.Task != "MOVE" && p.Task != "DELETE") {
		fail("batch request shape")
		return
	}
	token := s.Token
	if owner == "b" {
		token = TokenB
	}
	if r.Header.Get("X-YP-Access-Token") != token || r.Header.Get("pc-os") != "win" {
		fail("batch identity headers")
		return
	}
	if s.BatchCode != "" {
		jsonOut(w, map[string]any{"meta": map[string]string{"code": s.BatchCode, "message": "batch rejected"}, "result": s.BatchReply})
		return
	}
	if len(s.BatchReply) > 0 {
		rest(w, s.BatchReply)
		return
	}
	find := func(id string, typ int) (string, File, bool) {
		for parent, items := range s.Files {
			if !strings.HasPrefix(parent, owner+":") {
				continue
			}
			for _, item := range items {
				if item.ID == id && item.Type == typ {
					return parent, item, true
				}
			}
		}
		return "", File{}, false
	}
	if p.Task == "MOVE" {
		if p.TargetSpace != "0" || p.Target == "" {
			fail("move target space")
			return
		}
		if p.Target != "0" {
			if _, _, ok := find(p.Target, 0); !ok {
				fail("move target directory")
				return
			}
		}
	}
	selected := map[string]string{}
	for typ, ids := range [][]string{p.Dirs, p.Files} {
		for _, id := range ids {
			parent, item, ok := find(id, typ)
			if !ok || id == "0" || selected[id] != "" {
				fail("batch object identity/type")
				return
			}
			selected[id] = parent
			if typ == 1 && p.Task == "MOVE" && len(p.Files) == 1 && len(p.Dirs) == 0 && p.Version != strconv.Itoa(item.Version) {
				fail("move file version")
				return
			}
			if typ == 0 && p.Task == "DELETE" && len(s.Files[owner+":"+id]) > 0 {
				jsonOut(w, map[string]any{"meta": map[string]string{"code": "5056", "message": "nonempty directory"}})
				return
			}
		}
	}
	if (len(p.Files) != 1 || len(p.Dirs) > 0 || p.Task != "MOVE") && p.Version != "" {
		fail("unexpected move version")
		return
	}
	for id, parent := range selected {
		items := s.Files[parent]
		for i, item := range items {
			if item.ID == id {
				s.Files[parent] = append(items[:i:i], items[i+1:]...)
				if p.Task == "MOVE" {
					s.Files[owner+":"+p.Target] = append(s.Files[owner+":"+p.Target], item)
				} else if item.Type == 0 {
					delete(s.Files, owner+":"+id)
				}
				break
			}
		}
	}
	rest(w, map[string]any{"success": true, "finish": true, "taskId": fmt.Sprintf("batch-%d", s.Calls[r.URL.Path])})
}
