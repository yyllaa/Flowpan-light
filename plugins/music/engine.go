package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flowpan/app/pluginapi"
)

type engine struct {
	host  host
	now   func() time.Time
	calls int
}
type settings struct {
	Account   string `json:"account"`
	Directory string `json:"directory"`
	Source    string `json:"source"`
	Mode      string `json:"mode"`
	Conflict  string `json:"conflict"`
	Template  string `json:"filename_template"`
	Cover     bool   `json:"cover"`
	Lyrics    bool   `json:"lyrics"`
	Interval  int    `json:"interval"`
}
type batch struct {
	ID           string       `json:"id"`
	JobID        string       `json:"job_id"`
	Status       string       `json:"status"`
	Message      string       `json:"message"`
	Config       settings     `json:"config"`
	Policy       string       `json:"policy"`
	Started      int64        `json:"started"`
	Head         int          `json:"head"`
	Tail         int          `json:"tail"`
	Offset       int          `json:"offset"`
	Cursor       string       `json:"cursor,omitempty"`
	Chunks       int          `json:"chunks"`
	Chunk        int          `json:"chunk"`
	Within       int          `json:"within"`
	Total        int          `json:"total"`
	Done         int          `json:"done"`
	Written      int          `json:"written"`
	Skipped      int          `json:"skipped"`
	Review       int          `json:"review"`
	Current      *work        `json:"current,omitempty"`
	Recent       []itemResult `json:"recent,omitempty"`
	NextCloud    int64        `json:"next_cloud,omitempty"`
	ScanComplete bool         `json:"scan_complete"`
}
type work struct {
	Entry       pluginapi.StorageEntry  `json:"entry"`
	Fingerprint string                  `json:"fingerprint"`
	Phase       string                  `json:"phase"`
	Stage       string                  `json:"stage,omitempty"`
	Media       string                  `json:"media,omitempty"`
	Publish     string                  `json:"publish,omitempty"`
	Original    tags                    `json:"original"`
	Selected    *candidate              `json:"selected,omitempty"`
	Candidates  []candidate             `json:"candidates,omitempty"`
	Metadata    pluginapi.MediaMetadata `json:"metadata"`
	Provider    int                     `json:"provider"`
	Retry       int                     `json:"retry"`
	Generation  int                     `json:"generation"`
	Cover       string                  `json:"cover,omitempty"`
	ReviewIndex int                     `json:"review_index,omitempty"`
	NeedCover   bool                    `json:"need_cover"`
}
type itemResult struct {
	Index       int                   `json:"index"`
	Name        string                `json:"name"`
	ID          string                `json:"id"`
	Status      string                `json:"status"`
	Message     string                `json:"message,omitempty"`
	Fingerprint string                `json:"fingerprint"`
	Policy      string                `json:"policy"`
	Publish     string                `json:"publish,omitempty"`
	Backup      *pluginapi.FileObject `json:"backup,omitempty"`
	Candidates  []candidate           `json:"candidates,omitempty"`
	Work        *work                 `json:"work,omitempty"`
}
type deferred struct {
	until   int64
	message string
}

func (d *deferred) Error() string { return d.message }
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (e *engine) call(method string, in, out any) error {
	e.calls++
	return e.host.Call(method, in, out)
}
func (e *engine) get(key string, out any) error {
	return e.call("kv.get", map[string]any{"key": "music-" + digest(key)[:58]}, out)
}
func (e *engine) set(key string, value any) error {
	return e.call("kv.set", map[string]any{"key": "music-" + digest(key)[:58], "value": value}, nil)
}
func (e *engine) save(b *batch) error { return e.set("batch:"+b.ID, b) }
func (b *batch) key(s string) string  { return "music:" + b.ID + ":" + s }
func (b *batch) trackKey() string     { return b.key(fmt.Sprintf("track:%d:%d", b.Chunk, b.Within)) }
func (b *batch) operation(s string) string {
	return b.trackKey() + ":" + strconv.Itoa(b.Current.Generation) + ":" + s
}
func stateKey(account, id string) string { return "file:" + digest([]string{account, id}) }
func coverFormat(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3", ".flac", ".m4a", ".m4b":
		return true
	}
	return false
}
func validAudio(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3", ".flac", ".m4a", ".m4b", ".ogg", ".opus", ".wav":
		return !strings.Contains(name, "(flowpan-")
	}
	return false
}
func (e *engine) loadActive() (*batch, error) {
	var id string
	if err := e.get("active", &id); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	var b batch
	if err := e.get("batch:"+id, &b); err != nil {
		return nil, err
	}
	if b.ID == "" {
		return nil, nil
	}
	return &b, nil
}
func configOf(raw map[string]any) (settings, error) {
	c := settings{Source: "auto", Mode: "missing", Conflict: "keep_both", Template: "@2 - @1", Cover: true, Lyrics: true, Interval: 3}
	copy := make(map[string]any, len(raw))
	for key, value := range raw {
		copy[key] = value
	}
	switch accounts := raw["account"].(type) {
	case []any:
		if len(accounts) != 1 {
			return c, fmt.Errorf("请选择一个音乐来源")
		}
		copy["account"] = accounts[0]
	case []string:
		if len(accounts) != 1 {
			return c, fmt.Errorf("请选择一个音乐来源")
		}
		copy["account"] = accounts[0]
	}
	v, _ := json.Marshal(copy)
	if err := json.Unmarshal(v, &c); err != nil {
		return c, err
	}
	c.Account = strings.TrimSpace(c.Account)
	c.Directory = strings.TrimSpace(c.Directory)
	if c.Account == "" || c.Directory == "" {
		return c, fmt.Errorf("请先配置来源账号和音乐目录")
	}
	if c.Interval < 2 || c.Interval > 60 {
		return c, fmt.Errorf("网盘操作间隔为 2–60 秒")
	}
	if c.Mode != "missing" && c.Mode != "refresh" {
		return c, fmt.Errorf("标签策略无效")
	}
	if c.Conflict != "keep_both" && c.Conflict != "replace" {
		return c, fmt.Errorf("写入方式无效")
	}
	if c.Account == "local" && c.Conflict == "replace" {
		return c, fmt.Errorf("本地请选择保留原件并输出副本")
	}
	switch c.Source {
	case "auto", "netease", "qq", "itunes", "musicbrainz":
	default:
		return c, fmt.Errorf("音源无效")
	}
	return c, nil
}
func (e *engine) invoke(r pluginapi.Request) (pluginapi.Result, error) {
	b, err := e.loadActive()
	if err != nil {
		return pluginapi.Result{}, err
	}
	if r.ActionID == "status" {
		result := pluginapi.Result{Message: "尚未开始刮削", Data: map[string]any{"batch": b}}
		if b != nil {
			result.Message = b.Message
			if b.JobID != "" {
				var job any
				if err := e.call("task.status", map[string]any{"id": b.JobID}, &job); err == nil {
					result.Data["job"] = job
				}
			}
		}
		return result, nil
	}
	if r.ActionID == "records" {
		if b == nil {
			return pluginapi.Result{Data: map[string]any{"items": []itemResult{}, "total": 0}}, nil
		}
		offset := max(0, number(r.Input["offset"]))
		limit := 20
		items := []itemResult{}
		for n := offset + 1; n <= min(b.Done, offset+limit); n++ {
			var item itemResult
			if err := e.get(b.key("result:"+strconv.Itoa(n)), &item); err != nil {
				return pluginapi.Result{}, err
			}
			item.Work = nil
			items = append(items, item)
		}
		return pluginapi.Result{Data: map[string]any{"items": items, "total": b.Done, "offset": offset}}, nil
	}
	if r.ActionID == "restore" {
		if !r.Confirmed {
			return pluginapi.Result{}, fmt.Errorf("请先确认原件恢复")
		}
		var task pluginapi.FileTask
		id, _ := r.Input["publish_id"].(string)
		err := e.call("files.restore", map[string]any{"id": id, "key": "music-restore:" + id}, &task)
		if err != nil {
			return pluginapi.Result{}, err
		}
		result := pluginapi.Result{Message: task.Message, Data: map[string]any{"task": task}}
		if task.Status == "queued" || task.Status == "running" {
			result.Continuation = &pluginapi.Continuation{DelaySeconds: 2}
		} else if task.Status != "completed" {
			result.Failed = true
		} else if task.Result != nil {
			// A restored original must not inherit the processed-file shortcut.
			if err := e.set(stateKey(task.Result.Account, task.Result.ID), itemResult{Status: "restored"}); err != nil {
				return pluginapi.Result{}, err
			}
		}
		return result, nil
	}
	if !r.Confirmed {
		return pluginapi.Result{}, fmt.Errorf("请先确认刮削操作")
	}
	switch r.ActionID {
	case "start":
		if b == nil || b.Status == "completed" {
			c, err := configOf(r.Config)
			if err != nil {
				return pluginapi.Result{}, err
			}
			var caps pluginapi.HostCapabilities
			if err = e.call("host.capabilities", nil, &caps); err != nil {
				return pluginapi.Result{}, err
			}
			if caps.Limits.ContinuationSeconds == 0 || caps.Limits.MediaArtworkBytes == 0 {
				return pluginapi.Result{}, fmt.Errorf("请先更新宿主到支持分段任务与音乐封面的版本")
			}
			var media struct {
				FFmpeg  bool `json:"ffmpeg"`
				FFprobe bool `json:"ffprobe"`
			}
			if err = e.call("media.status", nil, &media); err != nil {
				return pluginapi.Result{}, err
			}
			if !media.FFmpeg || !media.FFprobe {
				return pluginapi.Result{}, fmt.Errorf("请先在设置中安装 FFmpeg 与 FFprobe")
			}
			var files pluginapi.FileCapabilities
			if err = e.call("files.capabilities", map[string]any{"account": c.Account}, &files); err != nil {
				return pluginapi.Result{}, err
			}
			if !files.Stage || !files.Publish {
				return pluginapi.Result{}, fmt.Errorf("来源需要读取、暂存及上传能力")
			}
			allowed := false
			for _, v := range files.Conflicts {
				allowed = allowed || v == c.Conflict
			}
			if !allowed {
				return pluginapi.Result{}, fmt.Errorf("来源未提供所选写入方式")
			}
			b = &batch{ID: digest([]string{r.TaskID, r.PluginID})[:24], JobID: r.TaskID, Status: "scanning", Config: c, Started: e.now().Unix(), Tail: 1}
			policy := c
			policy.Interval = 0
			b.Policy = digest(policy)
			if err = e.set(b.key("dir:0"), c.Directory); err != nil {
				return pluginapi.Result{}, err
			}
			if err = e.save(b); err != nil {
				return pluginapi.Result{}, err
			}
			if err = e.set("active", b.ID); err != nil {
				return pluginapi.Result{}, err
			}
			_ = e.call("log", map[string]any{"message": "音乐刮削开始；串行处理，间隔 " + strconv.Itoa(c.Interval) + " 秒"}, nil)
		} else if b.JobID != r.TaskID {
			return pluginapi.Result{Message: "已有刮削批次，请在音乐刮削页面查看或继续", Data: map[string]any{"batch": b}}, nil
		}
	case "resume":
		if b == nil {
			return pluginapi.Result{}, fmt.Errorf("没有可继续的刮削批次")
		}
		if b.JobID != r.TaskID {
			var old struct {
				Status string `json:"status"`
			}
			if err = e.call("task.status", map[string]any{"id": b.JobID}, &old); err != nil {
				return pluginapi.Result{}, err
			}
			if old.Status == "queued" || old.Status == "running" || old.Status == "cancelling" {
				return pluginapi.Result{Message: "原任务仍在执行", Data: map[string]any{"batch": b}}, nil
			}
			b.JobID = r.TaskID
		}
		if b.Status == "attention" {
			b.Status = "running"
			if !b.ScanComplete {
				b.Status = "scanning"
			}
			if b.Current != nil {
				b.Current.Retry = 0
			}
		}
	case "review":
		if b != nil && b.JobID == r.TaskID && b.Status == "running" {
			break
		}
		if b == nil || b.Status != "completed" {
			return pluginapi.Result{}, fmt.Errorf("请等待批次结束后确认待选条目")
		}
		n := number(r.Input["index"])
		choice := number(r.Input["candidate"])
		var item itemResult
		if err = e.get(b.key("review:"+strconv.Itoa(n)), &item); err != nil {
			return pluginapi.Result{}, err
		}
		if item.Work == nil || choice < 1 || choice > len(item.Candidates) {
			return pluginapi.Result{}, fmt.Errorf("请选择有效文件和候选")
		}
		b.Current = item.Work
		b.Current.ReviewIndex = n
		b.Current.Selected = &item.Candidates[choice-1]
		b.Current.Metadata = desired(b.Current.Original, *b.Current.Selected, b.Config.Mode)
		b.Current.Phase = "stage"
		b.Current.Generation++
		b.Current.Stage = ""
		b.Current.Media = ""
		b.Current.Publish = ""
		b.Current.Provider = 0
		b.Current.Retry = 0
		b.JobID = r.TaskID
		b.Status = "running"
		b.Chunk = b.Chunks
		b.Within = 0
		b.Total++
		b.Review = max(0, b.Review-1)
	default:
		return pluginapi.Result{}, fmt.Errorf("未知音乐刮削动作")
	}
	if b.Status == "completed" {
		return pluginapi.Result{Message: b.Message, Data: map[string]any{"batch": b}}, nil
	}
	delay := 1
	for step := 0; step < 4; step++ {
		err = e.step(b)
		if err != nil {
			var wait *deferred
			if errors.As(err, &wait) {
				delay = max(1, int(wait.until-e.now().Unix()))
				b.Message = wait.message
				err = nil
				break
			}
			b.Status = "attention"
			b.Message = err.Error()
			_ = e.save(b)
			_ = e.call("log", map[string]any{"message": "音乐刮削暂停：" + b.Message}, nil)
			return pluginapi.Result{Failed: true, Message: b.Message, Data: map[string]any{"batch": b}}, nil
		}
		if err = e.save(b); err != nil {
			return pluginapi.Result{}, err
		}
		if b.Status == "completed" || e.calls > 55 {
			break
		}
	}
	if err = e.save(b); err != nil {
		return pluginapi.Result{}, err
	}
	_ = e.call("task.progress", pluginapi.Progress{Done: int64(b.Done), Total: int64(b.Total), Message: b.Message}, nil)
	result := pluginapi.Result{Message: b.Message, Data: map[string]any{"batch": b}}
	if b.Status != "completed" {
		result.Continuation = &pluginapi.Continuation{DelaySeconds: min(86400, delay)}
	}
	return result, nil
}
func number(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}
func (e *engine) cloud(b *batch) error {
	if b.Config.Account == "local" {
		return nil
	}
	now := e.now().Unix()
	if now < b.NextCloud {
		return &deferred{b.NextCloud, "等待网盘操作间隔"}
	}
	b.NextCloud = now + int64(b.Config.Interval)
	return e.save(b)
}
func (e *engine) scan(b *batch) error {
	if b.Head >= b.Tail {
		b.ScanComplete = true
		b.Status = "running"
		b.Message = fmt.Sprintf("扫描完成，共 %d 首", b.Total)
		return nil
	}
	if err := e.cloud(b); err != nil {
		return err
	}
	var dir string
	if err := e.get(b.key("dir:"+strconv.Itoa(b.Head)), &dir); err != nil {
		return err
	}
	limit := min(30, max(1, (100-e.calls)/3))
	var page pluginapi.StoragePage
	if err := e.call("files.list", map[string]any{"account": b.Config.Account, "id": dir, "offset": b.Offset, "cursor": b.Cursor, "limit": limit}, &page); err != nil {
		return err
	}
	if len(page.Items) > limit {
		return fmt.Errorf("来源分页超过请求上限")
	}
	tracks := []pluginapi.StorageEntry{}
	for _, item := range page.Items {
		if item.ID == "" {
			return fmt.Errorf("来源返回空文件 ID")
		}
		if item.ParentID == "" {
			item.ParentID = dir
		}
		if item.IsDir {
			var visited int
			k := b.key("dir-seen:" + digest(item.ID))
			if err := e.get(k, &visited); err != nil {
				return err
			}
			if (visited > 0 && visited <= b.Tail) || item.ID == b.Config.Directory {
				continue
			}
			if b.Tail >= 5000 {
				return fmt.Errorf("目录数量超过 5000，请分目录运行")
			}
			if err := e.set(b.key("dir:"+strconv.Itoa(b.Tail)), item.ID); err != nil {
				return err
			}
			if err := e.set(k, b.Tail+1); err != nil {
				return err
			}
			b.Tail++
		} else if validAudio(item.Name) {
			tracks = append(tracks, item)
		}
	}
	if len(tracks) > 0 {
		if b.Total+len(tracks) > 10000 {
			return fmt.Errorf("单批次最多 10000 首，请分目录运行")
		}
		if err := e.set(b.key("chunk:"+strconv.Itoa(b.Chunks)), tracks); err != nil {
			return err
		}
		b.Chunks++
		b.Total += len(tracks)
	}
	if page.HasMore {
		if len(page.Items) == 0 || (page.NextCursor != "" && page.NextCursor == b.Cursor) {
			return fmt.Errorf("来源分页未推进，请检查目录列表")
		}
		b.Offset += len(page.Items)
		b.Cursor = page.NextCursor
	} else {
		b.Head++
		b.Offset = 0
		b.Cursor = ""
	}
	b.Message = fmt.Sprintf("扫描音乐目录，已找到 %d 首", b.Total)
	return nil
}
func (e *engine) next(b *batch) error {
	for b.Chunk < b.Chunks {
		var list []pluginapi.StorageEntry
		if err := e.get(b.key("chunk:"+strconv.Itoa(b.Chunk)), &list); err != nil {
			return err
		}
		if b.Within >= len(list) {
			b.Chunk++
			b.Within = 0
			continue
		}
		entry := list[b.Within]
		fingerprint := digest(entry)
		var previous itemResult
		if err := e.get(stateKey(b.Config.Account, entry.ID), &previous); err != nil {
			return err
		}
		if previous.Policy == b.Policy && previous.Fingerprint == fingerprint && (previous.Status == "written" || previous.Status == "unchanged" || previous.Status == "review") {
			previous.Index = b.Done + 1
			if previous.Status == "review" {
				if err := e.set(b.key("review:"+strconv.Itoa(previous.Index)), previous); err != nil {
					return err
				}
				b.Review++
			} else {
				previous.Status = "skipped"
				previous.Message = "文件未变化"
				b.Skipped++
			}
			if err := e.set(b.key("result:"+strconv.Itoa(previous.Index)), previous); err != nil {
				return err
			}
			summary := previous
			summary.Work = nil
			summary.Candidates = nil
			b.Recent = append(b.Recent, summary)
			if len(b.Recent) > 12 {
				b.Recent = b.Recent[len(b.Recent)-12:]
			}
			b.Done++
			b.Within++
			return nil
		}
		b.Current = &work{Entry: entry, Fingerprint: fingerprint, Phase: "stage"}
		return nil
	}
	b.Status = "completed"
	b.Message = fmt.Sprintf("刮削完成：写入 %d，跳过 %d，待确认 %d，共 %d 首", b.Written, b.Skipped, b.Review, b.Total)
	_ = e.call("log", map[string]any{"message": b.Message}, nil)
	_ = e.call("notify", map[string]any{"message": b.Message}, nil)
	return nil
}
func (e *engine) finish(b *batch, status, message string, published *pluginapi.FileTask) error {
	w := b.Current
	item := itemResult{Index: b.Done + 1, ID: w.Entry.ID, Name: w.Entry.Name, Status: status, Message: message, Fingerprint: w.Fingerprint, Policy: b.Policy, Publish: w.Publish}
	if published != nil {
		item.Backup = published.Backup
	}
	if status == "review" {
		item.Candidates = w.Candidates
		copy := *w
		item.Work = &copy
		if err := e.set(b.key("review:"+strconv.Itoa(item.Index)), item); err != nil {
			return err
		}
		b.Review++
	}
	if err := e.set(stateKey(b.Config.Account, w.Entry.ID), item); err != nil {
		return err
	}
	if published != nil && published.Result != nil {
		var entry pluginapi.StorageEntry
		if err := e.call("files.stat", map[string]any{"account": b.Config.Account, "id": published.Result.ID}, &entry); err != nil {
			return err
		}
		copy := item
		copy.ID = entry.ID
		copy.Fingerprint = digest(entry)
		if err := e.set(stateKey(b.Config.Account, entry.ID), copy); err != nil {
			return err
		}
	}
	if err := e.set(b.key("result:"+strconv.Itoa(item.Index)), item); err != nil {
		return err
	}
	if w.ReviewIndex > 0 && status != "review" {
		confirmed := item
		confirmed.Index = w.ReviewIndex
		confirmed.Candidates = nil
		confirmed.Work = nil
		if err := e.set(b.key("review:"+strconv.Itoa(w.ReviewIndex)), confirmed); err != nil {
			return err
		}
		if err := e.set(b.key("result:"+strconv.Itoa(w.ReviewIndex)), confirmed); err != nil {
			return err
		}
	}
	summary := item
	summary.Work, summary.Candidates = nil, nil
	b.Recent = append(b.Recent, summary)
	if len(b.Recent) > 12 {
		b.Recent = b.Recent[len(b.Recent)-12:]
	}
	if status == "written" {
		b.Written++
	} else if status != "review" {
		b.Skipped++
	}
	b.Done++
	b.Within++
	b.Current = nil
	b.Message = fmt.Sprintf("已处理 %d / %d，%s", b.Done, b.Total, item.Name)
	return nil
}
func (e *engine) step(b *batch) error {
	if b.Status == "scanning" {
		return e.scan(b)
	}
	if b.Current == nil {
		return e.next(b)
	}
	w := b.Current
	switch w.Phase {
	case "stage":
		if err := e.cloud(b); err != nil {
			return err
		}
		var task pluginapi.FileTask
		if err := e.call("files.stage", pluginapi.FileStage{Key: b.operation("stage"), Account: b.Config.Account, ID: w.Entry.ID}, &task); err != nil {
			return err
		}
		w.Stage = task.ID
		w.Phase = "stage-wait"
	case "stage-wait":
		var task pluginapi.FileTask
		if err := e.call("files.task", map[string]any{"id": w.Stage}, &task); err != nil {
			return err
		}
		switch task.Status {
		case "completed":
			w.Phase = "probe"
		case "interrupted", "failed", "cancelled":
			if err := e.call("files.release", map[string]any{"id": w.Stage}, nil); err != nil {
				return err
			}
			w.Generation++
			w.Phase = "stage"
			return fmt.Errorf("暂存中断，继续时重新下载当前文件")
		case "queued", "running":
			return &deferred{e.now().Unix() + 2, "正在暂存：" + w.Entry.Name}
		default:
			return fmt.Errorf("暂存状态需要核验：%s", task.Status)
		}
	case "probe":
		var raw json.RawMessage
		if err := e.call("media.probe", map[string]any{"file_task": w.Stage}, &raw); err != nil {
			return err
		}
		parsed, err := fromProbe(raw, w.Entry.Name, b.Config.Template)
		if err != nil {
			w.Candidates = nil
			w.Phase = "review-cleanup"
			return nil
		}
		w.Original = parsed
		v := parsed.Native
		if b.Config.Mode == "missing" && v.Title != "" && v.Artist != "" && v.Album != "" && v.AlbumArtist != "" && v.Date != "" && v.Genre != "" && v.Track != "" && v.Disc != "" && (!b.Config.Cover || parsed.HasCover || !coverFormat(w.Entry.Name)) && (!b.Config.Lyrics || v.Lyrics != "") {
			w.Phase = "unchanged-cleanup"
			return nil
		}
		w.NeedCover = coverFormat(w.Entry.Name) && b.Config.Cover && (!parsed.HasCover || b.Config.Mode == "refresh")
		if w.Selected != nil {
			w.Metadata = desired(parsed, *w.Selected, b.Config.Mode)
			w.Phase = "lyrics"
			return nil
		}
		w.Phase = "search"
	case "search":
		choices, accepted, err := e.search(b, w)
		if err != nil {
			return err
		}
		w.Candidates = choices
		if accepted {
			w.Selected = &choices[0]
			w.Metadata = desired(w.Original, choices[0], b.Config.Mode)
			w.Phase = "lyrics"
		} else {
			w.Phase = "review-cleanup"
		}
	case "lyrics":
		if b.Config.Lyrics && (w.Metadata.Lyrics == "" || b.Config.Mode == "refresh") {
			lyric, err := e.lyrics(*w.Selected)
			if err != nil {
				var wait *deferred
				if errors.As(err, &wait) {
					return err
				}
				_ = e.call("log", map[string]any{"message": "歌词未取得，保留现有歌词"}, nil)
			} else if len(lyric) <= 16384 {
				w.Metadata.Lyrics = lyric
			}
		}
		w.Phase = "cover"
	case "cover":
		if w.NeedCover && w.Selected.CoverURL != "" {
			id, err := e.cover(b, *w.Selected)
			if err != nil {
				var wait *deferred
				if errors.As(err, &wait) {
					return err
				}
				_ = e.call("log", map[string]any{"message": "封面未取得，保留现有封面"}, nil)
			} else {
				w.Cover = id
			}
		}
		if w.Metadata == w.Original.Native && w.Cover == "" {
			w.Phase = "unchanged-cleanup"
		} else {
			w.Phase = "media"
		}
	case "media":
		var task pluginapi.MediaTask
		p := pluginapi.MediaProcess{Key: b.operation("media"), FileTask: w.Stage, Operation: "remux", Format: strings.TrimPrefix(strings.ToLower(filepath.Ext(w.Entry.Name)), "."), Metadata: w.Metadata, CoverArtifact: w.Cover}
		if err := e.call("media.submit", p, &task); err != nil {
			return err
		}
		w.Media = task.ID
		w.Phase = "media-wait"
	case "media-wait":
		var task pluginapi.MediaTask
		if err := e.call("media.task", map[string]any{"id": w.Media}, &task); err != nil {
			return err
		}
		switch task.Status {
		case "completed":
			w.Phase = "publish"
		case "queued", "running":
			return &deferred{e.now().Unix() + 2, "正在无损封装：" + w.Entry.Name}
		case "interrupted", "failed", "cancelled":
			if err := e.call("media.release", map[string]any{"id": w.Media}, nil); err != nil {
				return err
			}
			w.Generation++
			w.Phase = "media"
			return fmt.Errorf("封装中断，继续时重建当前输出")
		default:
			return fmt.Errorf("媒体状态需要核验：%s", task.Status)
		}
	case "publish":
		if err := e.cloud(b); err != nil {
			return err
		}
		var task pluginapi.FileTask
		p := pluginapi.FilePublish{Key: b.operation("publish"), Account: b.Config.Account, ParentID: w.Entry.ParentID, Name: w.Entry.Name, MediaTask: w.Media, Conflict: b.Config.Conflict}
		if p.Conflict == "replace" {
			p.OriginalTask = w.Stage
		}
		if err := e.call("files.publish", p, &task); err != nil {
			return err
		}
		w.Publish = task.ID
		w.Phase = "publish-wait"
	case "publish-wait":
		var task pluginapi.FileTask
		if err := e.call("files.task", map[string]any{"id": w.Publish}, &task); err != nil {
			return err
		}
		if task.Status == "queued" || task.Status == "running" {
			return &deferred{e.now().Unix() + 2, "正在校验回写：" + w.Entry.Name}
		}
		if task.Status != "completed" && task.Status != "skipped" {
			return fmt.Errorf("回写需要核验：%s；保留任务 %s", task.Status, task.ID)
		}
		w.Phase = "cleanup"
	case "cleanup", "review-cleanup", "unchanged-cleanup":
		if w.Media != "" {
			if err := e.call("media.release", map[string]any{"id": w.Media}, nil); err != nil {
				return err
			}
		}
		if w.Stage != "" {
			if err := e.call("files.release", map[string]any{"id": w.Stage}, nil); err != nil {
				return err
			}
		}
		if w.Phase == "review-cleanup" {
			return e.finish(b, "review", "匹配未达到自动确认条件", nil)
		}
		if w.Phase == "unchanged-cleanup" {
			return e.finish(b, "unchanged", "标签与现有内容一致", nil)
		}
		var task pluginapi.FileTask
		if err := e.call("files.task", map[string]any{"id": w.Publish}, &task); err != nil {
			return err
		}
		return e.finish(b, "written", "已校验并回写", &task)
	default:
		return fmt.Errorf("刮削检查点无效")
	}
	b.Message = fmt.Sprintf("%d / %d · %s", b.Done, b.Total, w.Entry.Name)
	return nil
}
func retrySeconds(s string, now time.Time) int {
	n, err := strconv.Atoi(s)
	if err == nil && n > 0 {
		return min(86400, n)
	}
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return min(86400, max(1, int(math.Ceil(t.Sub(now).Seconds()))))
	}
	return 900
}
