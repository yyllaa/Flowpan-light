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
	var files, dirs []string
	version := ""
	for _, ref := range refs {
		current, err := a.find(ctx, ref.Parent, func(e wopan.Entry) bool { return e.ID == ref.ID && e.IsDir == ref.Dir })
		if err != nil {
			return err
		}
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
