package main

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"flowpan/app/pluginapi"
)

type remoteTask struct {
	ID      string     `json:"id"`
	State   int        `json:"state"`
	Error   string     `json:"error"`
	EndTime *time.Time `json:"end_time"`
}
type writeStep struct {
	ID     string       `json:"id"`
	From   string       `json:"from"`
	To     string       `json:"to"`
	Object object       `json:"object"`
	Sent   bool         `json:"sent"`
	Done   bool         `json:"done"`
	Tasks  []remoteTask `json:"tasks,omitempty"`
}
type writePlan struct {
	Steps []writeStep `json:"steps"`
}

func missing(err error) bool {
	var e *pluginapi.StorageError
	return errors.As(err, &e) && e.Status == 404
}
func (a *adapter) state(key string, v any) error {
	return hostCall("kv.set", map[string]any{"key": key, "value": v}, nil)
}
func (a *adapter) absent(p string) error {
	_, err := a.get(p)
	if missing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fail("conflict", 409, "目标已存在："+path.Base(p))
}
func (a *adapter) mutate() (any, error) {
	op := a.c.Operation
	p := a.c.Params
	if (op == "mkdir" || op == "rename") && !validName(p.Name) {
		return nil, fail("400", 400, "文件名无效")
	}
	key := "openlist-write-" + hash([]any{a.c.Account, a.namespace, op, p.ID, p.IDs, p.ParentID, p.Name})[:48]
	var plan *writePlan
	if err := hostCall("kv.get", map[string]any{"key": key}, &plan); err != nil {
		return nil, err
	}
	if plan == nil {
		plan = &writePlan{}
		parent := ""
		if op == "mkdir" || op == "move" || op == "copy" {
			var err error
			parent, err = a.resolve(p.ParentID)
			if err != nil {
				return nil, err
			}
			o, e := a.get(parent)
			if e != nil {
				return nil, e
			}
			if !o.IsDir {
				return nil, fail("400", 400, "目标不是目录")
			}
		}
		if op == "mkdir" {
			target := path.Join(parent, p.Name)
			if err := a.absent(target); err != nil {
				return nil, err
			}
			plan.Steps = []writeStep{{To: target, Object: object{Name: p.Name, IsDir: true}}}
		} else {
			ids := p.IDs
			if op == "rename" {
				ids = []string{p.ID}
			}
			if len(ids) == 0 || len(ids) > 8 {
				return nil, fail("400", 400, "单次文件操作支持 1 至 8 项")
			}
			seen := map[string]bool{}
			for _, id := range ids {
				if seen[id] {
					continue
				}
				seen[id] = true
				from, err := a.resolve(id)
				if err != nil {
					return nil, err
				}
				if from == a.root {
					return nil, fail("403", 403, "不能修改账号根目录")
				}
				o, err := a.get(from)
				if err != nil {
					return nil, err
				}
				step := writeStep{ID: id, From: from, Object: o}
				if op == "rename" {
					step.To = path.Join(path.Dir(from), p.Name)
				} else if op != "delete" {
					step.To = path.Join(parent, path.Base(from))
				}
				if step.To == from {
					step.Done = true
				} else if step.To != "" {
					if strings.HasPrefix(step.To, from+"/") {
						return nil, fail("400", 400, "不能移动到自身目录")
					}
					if err = a.absent(step.To); err != nil {
						return nil, err
					}
				}
				plan.Steps = append(plan.Steps, step)
			}
		}
		if err := a.state(key, plan); err != nil {
			return nil, err
		}
	}
	var last any
	for n := range plan.Steps {
		s := &plan.Steps[n]
		if s.Done {
			continue
		}
		if !s.Sent {
			// Persist uncertainty before submitting a non-idempotent operation.
			s.Sent = true
			if err := a.state(key, plan); err != nil {
				return nil, err
			}
			var body any
			route := op
			switch op {
			case "mkdir":
				body = map[string]any{"path": s.To}
			case "rename":
				body = map[string]any{"path": s.From, "name": path.Base(s.To)}
			case "delete":
				route = "remove"
				body = map[string]any{"dir": path.Dir(s.From), "names": []string{path.Base(s.From)}}
			case "move", "copy":
				body = map[string]any{"src_dir": path.Dir(s.From), "dst_dir": path.Dir(s.To), "names": []string{path.Base(s.From)}, "overwrite": false}
			}
			var receipt struct {
				Tasks []remoteTask `json:"tasks"`
			}
			if err := a.api("POST", "/api/fs/"+route, body, &receipt); err != nil {
				var detail *pluginapi.StorageError
				if errors.As(err, &detail) && detail.Status >= 400 && detail.Status < 500 {
					s.Sent = false
					_ = a.state(key, plan)
				}
				return nil, err
			}
			s.Tasks = receipt.Tasks
			if err := a.state(key, plan); err != nil {
				return nil, err
			}
		}
		if err := a.waitTasks(op, s.Tasks); err != nil {
			return nil, err
		}
		var final object
		var err error
		if op == "delete" {
			_, err = a.get(s.From)
			if !missing(err) {
				if err != nil {
					return nil, err
				}
				return nil, fail("operation_pending", 409, "删除尚未确认，请重试核验")
			}
			if _, err = a.paths("forget", nil, s.ID, ""); err != nil {
				return nil, err
			}
		} else {
			final, err = a.get(s.To)
			if err != nil {
				return nil, err
			}
			if final.IsDir != s.Object.IsDir || !final.IsDir && final.Size != s.Object.Size {
				return nil, fail("verification_failed", 409, "文件操作结果未通过大小或类型核验")
			}
			if op == "move" || op == "rename" {
				_, err = a.get(s.From)
				if !missing(err) {
					if err != nil {
						return nil, err
					}
					return nil, fail("operation_pending", 409, "原路径仍存在，请重试核验")
				}
				if _, err = a.paths("relocate", nil, s.ID, s.To); err != nil {
					return nil, err
				}
			}
			last, err = a.entry(s.To, final)
			if err != nil {
				return nil, err
			}
		}
		s.Done = true
		if err = a.state(key, plan); err != nil {
			return nil, err
		}
	}
	if err := a.state(key, nil); err != nil {
		return nil, err
	}
	return last, nil
}
func (a *adapter) waitTasks(op string, tasks []remoteTask) error {
	if len(tasks) == 0 {
		return nil
	}
	if op != "copy" && op != "move" {
		return fmt.Errorf("异步任务类型无效")
	}
	for _, saved := range tasks {
		complete := false
		for n := 0; n < 12; n++ {
			var current remoteTask
			if err := a.api("POST", "/api/task/"+op+"/info?tid="+saved.ID, nil, &current); err != nil {
				return err
			}
			if current.Error != "" {
				return fail("task_failed", 0, "OpenList 任务失败："+current.Error)
			}
			if current.State == 2 {
				complete = true
				break
			}
			if current.State == 7 || current.State == 4 {
				return fail("task_failed", 0, "OpenList 任务已失败或取消")
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !complete {
			return fail("operation_pending", 409, "OpenList 任务仍在执行，再次操作将继续核验任务 "+saved.ID)
		}
	}
	return nil
}
