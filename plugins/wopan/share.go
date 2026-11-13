package main

import (
	"context"
	"errors"
	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
)

func (a *adapter) share(ctx context.Context) (any, error) {
	p := a.call.Params
	if p.Share == nil {
		return nil, errors.New("缺少联通分享参数")
	}
	if a.call.Operation == "share.resolve" {
		s, err := a.client.ResolveShare(ctx, p.Share.URL, p.Share.Password)
		if err != nil {
			return nil, err
		}
		info := pluginapi.StorageShareInfo{ID: s.ID, Name: s.Files[0].Name, Entries: []pluginapi.StorageShareEntry{}, Context: map[string]string{"client_id": s.ClientID, "code": s.Code}}
		for _, e := range s.Files {
			info.Entries = append(info.Entries, pluginapi.StorageShareEntry{ID: e.ID, Name: e.Name, IsDir: e.Type == "0" || e.Type == float64(0)})
		}
		return info, nil
	}
	if a.call.Operation != "share.save" {
		return nil, errors.New("联通分享返回同步结果，无需轮询")
	}
	if err := a.ready(ctx); err != nil {
		return nil, err
	}
	parent, err := decodeRef(p.ParentID)
	if err != nil || !parent.Dir {
		return nil, errors.New("请选择联通云盘目录")
	}
	info := p.Share.Info
	if info == nil || info.ID == "" || len(info.Entries) == 0 {
		return nil, errors.New("请先解析联通分享")
	}
	s := wopan.Share{ID: info.ID, ClientID: info.Context["client_id"], Code: info.Context["code"]}
	for _, e := range info.Entries {
		kind := "1"
		if e.IsDir {
			kind = "0"
		}
		s.Files = append(s.Files, wopan.ShareFile{ID: e.ID, Name: e.Name, Type: kind})
	}
	if err = a.client.ReceiveShare(ctx, s, parent.ID); err != nil {
		return nil, err
	}
	return pluginapi.StorageShareReceipt{Done: true, Count: len(s.Files), Message: "转存完成"}, nil
}
