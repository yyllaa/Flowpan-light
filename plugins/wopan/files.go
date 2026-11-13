package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
)

type objectRef struct {
	ID        string   `json:"i"`
	FID       string   `json:"f,omitempty"`
	Parent    string   `json:"p"`
	Dir       bool     `json:"d,omitempty"`
	Ancestors []string `json:"a,omitempty"`
}

func encodeRef(r objectRef) string {
	if r.ID == "0" && r.Dir {
		return rootID
	}
	b, _ := json.Marshal(r)
	return "wo1:" + base64.RawURLEncoding.EncodeToString(b)
}
func decodeRef(s string) (objectRef, error) {
	if s == rootID {
		return objectRef{ID: "0", Dir: true}, nil
	}
	if !strings.HasPrefix(s, "wo1:") || len(s) > 16384 {
		return objectRef{}, errors.New("联通云盘文件标识无效")
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, "wo1:"))
	var r objectRef
	if err != nil || json.Unmarshal(b, &r) != nil || r.ID == "" || r.ID == "0" || r.Parent == "" || len(r.ID) > 4096 || len(r.FID) > 4096 || len(r.Ancestors) == 0 || len(r.Ancestors) > 128 || r.Ancestors[0] != "0" || r.Ancestors[len(r.Ancestors)-1] != r.Parent {
		return r, errors.New("联通云盘文件标识无效")
	}
	return r, nil
}
func entry(e wopan.Entry, parent objectRef) pluginapi.StorageEntry {
	ancestors := append(append([]string(nil), parent.Ancestors...), parent.ID)
	id := encodeRef(objectRef{ID: e.ID, FID: e.FID, Parent: parent.ID, Dir: e.IsDir, Ancestors: ancestors})
	parentID := encodeRef(parent)
	var modified int64
	for _, layout := range []string{time.RFC3339, "20060102150405", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, e.ModTime, time.FixedZone("UTC+8", 8*3600)); err == nil {
			modified = t.Unix()
			break
		}
	}
	if modified == 0 {
		if n, err := strconv.ParseInt(e.ModTime, 10, 64); err == nil {
			if n > 1e12 {
				n /= 1000
			}
			if n > 0 && n < 1e11 {
				modified = n
			}
		}
	}
	return pluginapi.StorageEntry{ID: id, ParentID: parentID, Name: e.Name, IsDir: e.IsDir, Size: e.Size, Modified: modified, Version: e.FileVersion}
}

type cursor struct {
	Parent  string `json:"parent"`
	Offset  int    `json:"offset"`
	Limit   int    `json:"limit"`
	Sort    int    `json:"sort"`
	Keyword string `json:"keyword"`
}

func sortRule(order string, asc bool) int {
	base := 1
	switch order {
	case "size":
		base = 3
	case "mtime", "time", "modified":
		base = 5
	}
	if !asc {
		base++
	}
	return base
}
func (a *adapter) list(ctx context.Context, p pluginapi.StorageParams) (pluginapi.StoragePage, error) {
	parent, err := decodeRef(p.ParentID)
	if err != nil || !parent.Dir {
		return pluginapi.StoragePage{}, errors.New("请选择联通云盘目录")
	}
	size := p.Limit
	if size <= 0 {
		size = 200
	}
	if size > 500 {
		size = 500
	}
	off := p.Offset
	sort := sortRule(p.Order, p.Asc)
	if p.Cursor != "" {
		var c cursor
		b, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		if err != nil || json.Unmarshal(b, &c) != nil || c.Parent != p.ParentID || c.Limit != size || c.Sort != sort || c.Keyword != p.Keyword {
			return pluginapi.StoragePage{}, errors.New("联通云盘分页游标无效")
		}
		off = c.Offset
	}
	if off < 0 || off/size > 1000000 {
		return pluginapi.StoragePage{}, errors.New("联通云盘分页位置无效")
	}
	list, more, err := a.client.ListPage(ctx, parent.ID, off/size, size, sort)
	if err != nil {
		return pluginapi.StoragePage{}, err
	}
	out := pluginapi.StoragePage{Items: []pluginapi.StorageEntry{}, HasMore: more, Offset: off, Limit: size, Total: -1}
	start := off % size
	if start > len(list) {
		start = len(list)
	}
	for _, e := range list[start:] {
		if e.ID == "" || e.Name == "" {
			return out, errors.New("联通云盘目录返回无效文件")
		}
		if p.Keyword != "" && !strings.Contains(strings.ToLower(e.Name), strings.ToLower(p.Keyword)) {
			continue
		}
		out.Items = append(out.Items, entry(e, parent))
	}
	if more {
		b, _ := json.Marshal(cursor{p.ParentID, (off/size + 1) * size, size, sort, p.Keyword})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		out.Total = (off/size)*size + len(list)
	}
	return out, nil
}
func (a *adapter) find(ctx context.Context, parent string, match func(wopan.Entry) bool) (wopan.Entry, error) {
	if parent == "" {
		return wopan.Entry{}, errors.New("联通云盘文件缺少父目录")
	}
	for page := 0; page < 1000; page++ {
		items, more, err := a.client.ListPage(ctx, parent, page, 500, 1)
		if err != nil {
			return wopan.Entry{}, err
		}
		for _, e := range items {
			if match(e) {
				return e, nil
			}
		}
		if !more {
			return wopan.Entry{}, errors.New("联通云盘文件不存在或已移动")
		}
	}
	return wopan.Entry{}, errors.New("联通云盘目录分页超过上限")
}
func (a *adapter) stat(ctx context.Context, id string) (pluginapi.StorageEntry, error) {
	ref, err := decodeRef(id)
	if err != nil {
		return pluginapi.StorageEntry{}, err
	}
	if id == rootID {
		return pluginapi.StorageEntry{ID: rootID, Name: "联通云盘", Path: "/", IsDir: true}, nil
	}
	e, err := a.find(ctx, ref.Parent, func(e wopan.Entry) bool { return e.ID == ref.ID && e.IsDir == ref.Dir })
	if err != nil {
		return pluginapi.StorageEntry{}, err
	}
	parent := objectRef{ID: ref.Parent, Dir: true}
	if len(ref.Ancestors) > 1 {
		parent.Ancestors = append([]string(nil), ref.Ancestors[:len(ref.Ancestors)-1]...)
		parent.Parent = parent.Ancestors[len(parent.Ancestors)-1]
	}
	result := entry(e, parent)
	result.ID = id
	return result, nil
}
