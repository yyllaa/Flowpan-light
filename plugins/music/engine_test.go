package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"flowpan/app/pluginapi"
)

type fixtureHost struct {
	state            map[string]json.RawMessage
	entries          []pluginapi.StorageEntry
	byID             map[string]pluginapi.StorageEntry
	calls            map[string]int
	tasks            map[string]pluginapi.FileTask
	media            map[string]pluginapi.MediaTask
	now              time.Time
	active, peak     int
	responseStatus   int
	responseBody     string
	lastRequest      map[string]any
	publishAttention bool
}

func fixture(n int) *fixtureHost {
	h := &fixtureHost{state: map[string]json.RawMessage{}, byID: map[string]pluginapi.StorageEntry{}, calls: map[string]int{}, tasks: map[string]pluginapi.FileTask{}, media: map[string]pluginapi.MediaTask{}, now: time.Unix(1800000000, 0)}
	for i := 0; i < n; i++ {
		entry := pluginapi.StorageEntry{ID: fmt.Sprint(i), ParentID: "root", Name: fmt.Sprintf("Artist - Song%03d.flac", i%100), Size: 1024, Modified: 123, Version: "v1"}
		h.entries = append(h.entries, entry)
		h.byID[entry.ID] = entry
	}
	return h
}
func assign(value, out any) error {
	if out == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
func (h *fixtureHost) Call(method string, in, out any) error {
	h.calls[method]++
	p := map[string]any{}
	data, _ := json.Marshal(in)
	_ = json.Unmarshal(data, &p)
	switch method {
	case "kv.get":
		return assign(h.state[p["key"].(string)], out)
	case "kv.set":
		key := p["key"].(string)
		if len(key) > 64 || strings.Trim(key, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			return fmt.Errorf("invalid host state key %q", key)
		}
		raw, _ := json.Marshal(p["value"])
		if len(raw) > 60000 {
			return fmt.Errorf("oversize state %d", len(raw))
		}
		h.state[p["key"].(string)] = raw
		return nil
	case "host.capabilities":
		return assign(pluginapi.HostCapabilities{Limits: pluginapi.RuntimeLimits()}, out)
	case "media.status":
		return assign(map[string]bool{"ffmpeg": true, "ffprobe": true}, out)
	case "files.capabilities":
		return assign(pluginapi.FileCapabilities{Stage: true, Publish: true, Conflicts: []string{"keep_both", "replace"}}, out)
	case "files.list":
		start := number(p["offset"])
		limit := number(p["limit"])
		end := min(len(h.entries), start+limit)
		return assign(pluginapi.StoragePage{Items: h.entries[start:end], Offset: start, Limit: limit, Total: len(h.entries), HasMore: end < len(h.entries)}, out)
	case "files.stat":
		return assign(h.byID[p["id"].(string)], out)
	case "files.stage":
		key := p["key"].(string)
		id := "stage:" + key
		if old, ok := h.tasks[id]; ok {
			return assign(old, out)
		}
		h.active++
		h.peak = max(h.peak, h.active)
		var request pluginapi.FileStage
		_ = json.Unmarshal(data, &request)
		task := pluginapi.FileTask{ID: id, Status: "completed", Stage: &request}
		h.tasks[id] = task
		return assign(task, out)
	case "files.restore":
		return assign(h.tasks["restore"], out)
	case "files.task":
		return assign(h.tasks[p["id"].(string)], out)
	case "media.probe":
		task := h.tasks[p["file_task"].(string)]
		entry := h.byID[task.Stage.ID]
		title := strings.TrimSuffix(strings.TrimPrefix(entry.Name, "Artist - "), ".flac")
		return assign(map[string]any{"format": map[string]any{"duration": "180", "tags": map[string]string{"title": title, "artist": "Artist"}}, "streams": []any{map[string]any{"codec_type": "audio"}}}, out)
	case "media.submit":
		var request pluginapi.MediaProcess
		_ = json.Unmarshal(data, &request)
		id := "media:" + request.Key
		task := pluginapi.MediaTask{ID: id, Status: "completed", Request: request}
		h.media[id] = task
		return assign(task, out)
	case "media.task":
		return assign(h.media[p["id"].(string)], out)
	case "files.publish":
		var request pluginapi.FilePublish
		_ = json.Unmarshal(data, &request)
		id := "publish:" + request.Key
		if old, ok := h.tasks[id]; ok {
			return assign(old, out)
		}
		input := h.tasks[h.media[request.MediaTask].Request.FileTask]
		original := h.byID[input.Stage.ID]
		entry := original
		entry.ID = "result:" + id
		entry.Name = strings.TrimSuffix(original.Name, ".flac") + " (flowpan-copy-abc).flac"
		entry.Version = "tagged"
		h.byID[entry.ID] = entry
		task := pluginapi.FileTask{ID: id, Status: "completed", Result: &pluginapi.FileObject{ID: entry.ID, Name: entry.Name, Size: entry.Size}, Publish: &request}
		if h.publishAttention {
			task.Status = "attention"
		}
		h.tasks[id] = task
		return assign(task, out)
	case "files.release":
		id := p["id"].(string)
		task := h.tasks[id]
		if task.Status != "released" {
			h.active--
			task.Status = "released"
			h.tasks[id] = task
		}
		return nil
	case "media.release":
		return nil
	case "http.fetch":
		h.lastRequest = p
		status := h.responseStatus
		if status == 0 {
			status = 200
		}
		body := h.responseBody
		if body == "" {
			var query string
			for _, entry := range h.entries {
				title := strings.TrimSuffix(strings.TrimPrefix(entry.Name, "Artist - "), ".flac")
				if strings.Contains(p["url"].(string), title) {
					query = title
					break
				}
			}
			body = fmt.Sprintf(`{"resultCount":1,"results":[{"trackId":123,"trackName":%q,"artistName":"Artist","collectionName":"Album","trackTimeMillis":180000,"trackNumber":1}]}`, query)
		}
		return assign(map[string]any{"status": status, "body": body, "headers": map[string]string{"Retry-After": "17"}}, out)
	case "task.status":
		return assign(map[string]any{"status": "failed"}, out)
	case "task.progress", "log", "notify":
		return nil
	}
	return fmt.Errorf("unhandled host call %s", method)
}
func request(id string) pluginapi.Request {
	return pluginapi.Request{PluginID: "music-scraper", TaskID: id, ActionID: "start", Confirmed: true, Config: map[string]any{"account": "cloud", "directory": "root", "source": "itunes", "cover": false, "lyrics": false, "mode": "missing", "interval": 3}}
}
func drive(t *testing.T, h *fixtureHost, r pluginapi.Request) (pluginapi.Result, int) {
	t.Helper()
	maxCalls := 0
	for step := 0; step < 100000; step++ {
		e := &engine{host: h, now: func() time.Time { return h.now }}
		result, err := e.invoke(r)
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed {
			t.Fatalf("failed: %s", result.Message)
		}
		maxCalls = max(maxCalls, e.calls)
		if e.calls > pluginapi.MaxHostCalls {
			t.Fatalf("host call budget exceeded %d", e.calls)
		}
		if result.Continuation == nil {
			return result, maxCalls
		}
		h.now = h.now.Add(time.Duration(result.Continuation.DelaySeconds) * time.Second)
	}
	t.Fatal("batch did not finish")
	return pluginapi.Result{}, 0
}
func Test3000TracksBoundedIncrementalAndMetadataCache(t *testing.T) {
	h := fixture(3000)
	r := request("first")
	result, maxCalls := drive(t, h, r)
	b := result.Data["batch"].(*batch)
	if b.Done != 3000 || b.Written != 3000 || b.Review != 0 || h.calls["http.fetch"] != 100 || h.peak != 1 || h.active != 0 {
		t.Fatalf("unexpected batch %+v fetch=%d peak=%d active=%d", b, h.calls["http.fetch"], h.peak, h.active)
	}
	stages, writes, queries := h.calls["files.stage"], h.calls["files.publish"], h.calls["http.fetch"]
	result, _ = drive(t, h, request("second"))
	b = result.Data["batch"].(*batch)
	if b.Done != 3000 || b.Skipped != 3000 || h.calls["files.stage"] != stages || h.calls["files.publish"] != writes || h.calls["http.fetch"] != queries {
		t.Fatalf("incremental rerun repeated work: %+v", b)
	}
	t.Logf("MUSIC_3000_PASS files=3000 metadata_requests=100 peak_staged_files=1 released=3000 max_calls_per_invocation=%d rerun_downloads=0 rerun_queries=0 rerun_uploads=0", maxCalls)
}
func TestResumeUnknownPublicationNeverUploadsAgain(t *testing.T) {
	h := fixture(1)
	h.publishAttention = true
	r := request("first")
	for step := 0; step < 40; step++ {
		e := &engine{host: h, now: func() time.Time { return h.now }}
		result, err := e.invoke(r)
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed {
			break
		}
		if result.Continuation != nil {
			h.now = h.now.Add(time.Duration(result.Continuation.DelaySeconds) * time.Second)
		}
	}
	before := h.calls["files.publish"]
	for id, task := range h.tasks {
		if task.Publish != nil {
			task.Status = "completed"
			h.tasks[id] = task
		}
	}
	r.TaskID = "resume"
	r.ActionID = "resume"
	result, _ := drive(t, h, r)
	if h.calls["files.publish"] != before || result.Data["batch"].(*batch).Written != 1 || h.active != 0 {
		t.Fatal("resume replayed publication or leaked a stage")
	}
}
func TestRateLimitSurvivesNewInvocationAndHonorsRetryAfter(t *testing.T) {
	h := fixture(0)
	h.responseStatus = 429
	h.responseBody = `{"error":"limit"}`
	for attempt := 0; attempt < 2; attempt++ {
		e := &engine{host: h, now: func() time.Time { return h.now }}
		_, err := e.fetch("itunes", structRequest())
		wait, ok := err.(*deferred)
		if !ok || wait.until != h.now.Unix()+17 {
			t.Fatalf("wrong cooldown %v", err)
		}
	}
	if h.calls["http.fetch"] != 1 {
		t.Fatal("rate limit was not persisted")
	}
}
func TestLowConfidenceAndChangedFile(t *testing.T) {
	h := fixture(2)
	h.responseBody = `{"resultCount":1,"results":[{"trackId":123,"trackName":"Different song","artistName":"Other","collectionName":"Album"}]}`
	result, _ := drive(t, h, request("first"))
	b := result.Data["batch"].(*batch)
	if b.Review != 2 || h.calls["files.publish"] != 0 || h.active != 0 {
		t.Fatal("ambiguous match wrote files")
	}
	h.responseBody = ""
	h.now = h.now.Add(31 * 24 * time.Hour)
	h.entries[0].Version = "changed"
	h.byID[h.entries[0].ID] = h.entries[0]
	result, _ = drive(t, h, request("second"))
	if result.Data["batch"].(*batch).Written != 1 {
		t.Fatal("changed tracks not rescanned")
	}
}

func TestIncrementalRecordsAndReviewCache(t *testing.T) {
	h := fixture(2)
	drive(t, h, request("first"))
	before := h.calls["files.stage"]
	result, _ := drive(t, h, request("second"))
	if result.Data["batch"].(*batch).Skipped != 2 || h.calls["files.stage"] != before {
		t.Fatal("repeat stage")
	}
	r := request("records")
	r.ActionID = "records"
	e := &engine{host: h, now: func() time.Time { return h.now }}
	records, err := e.invoke(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range records.Data["items"].([]itemResult) {
		if item.Name == "" || item.Status != "skipped" {
			t.Fatal("missing incremental record", item)
		}
	}
	h = fixture(1)
	h.responseBody = `{"resultCount":1,"results":[{"trackId":123,"trackName":"Song000 (Live)","artistName":"Artist","collectionName":"Album"}]}`
	drive(t, h, request("first"))
	before = h.calls["files.stage"]
	result, _ = drive(t, h, request("second"))
	if result.Data["batch"].(*batch).Review != 1 || h.calls["files.stage"] != before {
		t.Fatal("review downloaded twice")
	}
	r = request("review")
	r.ActionID = "review"
	r.Input = map[string]any{"index": 1, "candidate": 1}
	result, _ = drive(t, h, r)
	if result.Data["batch"].(*batch).Written != 1 {
		t.Fatal("manual review continuation failed")
	}
	r.TaskID = "duplicate"
	e = &engine{host: h, now: func() time.Time { return h.now }}
	if _, err = e.invoke(r); err == nil {
		t.Fatal("same review accepted twice")
	}
}

func TestRestoreInvalidatesOriginalIncrementalShortcut(t *testing.T) {
	h := fixture(1)
	e := &engine{host: h, now: func() time.Time { return h.now }}
	if err := e.set(stateKey("local", "0"), itemResult{Status: "written", Policy: "old"}); err != nil {
		t.Fatal(err)
	}
	h.tasks["restore"] = pluginapi.FileTask{Status: "completed", Result: &pluginapi.FileObject{Account: "local", ID: "0"}}
	r := request("restore")
	r.ActionID = "restore"
	r.Input = map[string]any{"publish_id": "receipt"}
	if _, err := e.invoke(r); err != nil {
		t.Fatal(err)
	}
	var prior itemResult
	if err := e.get(stateKey("local", "0"), &prior); err != nil || prior.Status != "restored" {
		t.Fatal("restored original was still marked processed", err, prior)
	}
}
