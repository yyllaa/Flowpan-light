package main

import (
	"context"
	"errors"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
)

// Validate the whole selection before the first write, including entries that
// may have moved since their opaque references were obtained.
func (a *adapter) mutate(ctx context.Context, op string, p pluginapi.StorageParams) error {
	if len(p.IDs) == 0 || len(p.IDs) > 1000 {
		return errors.New("请选择 1 至 1000 个联通云盘文件")
	}
	refs := make([]objectRef, 0, len(p.IDs))
	seen := map[string]bool{}
	for _, id := range p.IDs {
		ref, err := decodeRef(id)
		if err != nil {
			return err
		}
		if ref.ID == "0" {
			return errors.New("根目录不支持删除或移动")
		}
		if !seen[ref.ID] {
			seen[ref.ID] = true
			refs = append(refs, ref)
		}
	}
	var target objectRef
	if op == "move" {
		var err error
		target, err = decodeRef(p.ParentID)
		if err != nil || !target.Dir {
			return errors.New("请选择联通云盘目录")
		}
		for _, ref := range refs {
			if !ref.Dir {
				continue
			}
			if target.ID == ref.ID {
				return errors.New("目录不能移动到自身或子目录")
			}
			for _, id := range target.Ancestors {
				if id == ref.ID {
					return errors.New("目录不能移动到自身或子目录")
				}
			}
		}
		if _, err = a.stat(ctx, p.ParentID); err != nil {
			return err
		}
	}
	// Read each parent only once for this operation; do not retain stale listings.
	pending := make(map[string]map[string]objectRef)
	parents := make([]string, 0)
	for _, ref := range refs {
		if pending[ref.Parent] == nil {
			pending[ref.Parent] = make(map[string]objectRef)
			parents = append(parents, ref.Parent)
		}
		pending[ref.Parent][ref.ID] = ref
	}
	found := make(map[string]wopan.Entry, len(refs))
	for _, parent := range parents {
		remaining := pending[parent]
		for page := 0; len(remaining) > 0; page++ {
			if page >= 1000 {
				return errors.New("联通云盘目录分页超过上限")
			}
			items, more, err := a.client.ListPage(ctx, parent, page, 500, 1)
			if err != nil {
				return err
			}
			for _, e := range items {
				if ref, ok := remaining[e.ID]; ok && e.IsDir == ref.Dir {
					found[e.ID] = e
					delete(remaining, e.ID)
				}
			}
			if len(remaining) > 0 && !more {
				return errors.New("联通云盘文件不存在或已移动")
			}
		}
	}
	var files, dirs []string
	version := ""
	for _, ref := range refs {
		current := found[ref.ID]
		if op == "move" && ref.Parent == target.ID {
			continue
		}
		if ref.Dir {
			dirs = append(dirs, ref.ID)
		} else {
			files = append(files, ref.ID)
			version = current.FileVersion
		}
	}
	if len(files)+len(dirs) == 0 {
		return nil
	}
	if err := waitWrite(ctx); err != nil {
		return err
	}
	if op == "move" {
		return a.client.Move(ctx, wopan.SpacePersonal, files, dirs, target.ID, "", "", version)
	}
	return a.client.Delete(ctx, wopan.SpacePersonal, files, dirs)
}
