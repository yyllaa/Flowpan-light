package main

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"flowpan/app/pluginapi"
)

type uploadCheckpoint struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Namespace string `json:"namespace"`
}

func (a *adapter) upload() (pluginapi.StorageUploadStep, error) {
	p := a.c.Params
	input := p.Upload
	if input == nil || input.Size < 0 || !validName(p.Name) {
		return pluginapi.StorageUploadStep{}, fail("400", 400, "上传参数无效")
	}
	parent, err := a.resolve(p.ParentID)
	if err != nil {
		return pluginapi.StorageUploadStep{}, err
	}
	target := path.Join(parent, p.Name)
	var checkpoint uploadCheckpoint
	if len(input.Checkpoint) > 0 && string(input.Checkpoint) != "null" {
		if err = json.Unmarshal(input.Checkpoint, &checkpoint); err != nil {
			return pluginapi.StorageUploadStep{}, err
		}
		if checkpoint.Path != target || checkpoint.Size != input.Size || checkpoint.Namespace != a.namespace {
			return pluginapi.StorageUploadStep{}, fail("checkpoint_invalid", 409, "上传检查点与目标不一致")
		}
		if input.Response == nil {
			// A missing receipt can only be recovered using a matching remote hash.
			o, e := a.get(target)
			if e == nil && o.Size == input.Size && !o.IsDir {
				for algorithm, value := range o.HashInfo {
					if strings.EqualFold(strings.ReplaceAll(algorithm, "-", ""), "sha256") && checkpoint.SHA256 != "" && strings.EqualFold(value, checkpoint.SHA256) {
						entry, e := a.entry(target, o)
						return pluginapi.StorageUploadStep{Entry: &entry, Reused: true}, e
					}
				}
			}
			return pluginapi.StorageUploadStep{}, fail("upload_receipt_unknown", 409, "上传回执缺失，保留检查点等待核验，不重复上传")
		}
		if err = decodeResponse(*input.Response, nil); err != nil {
			return pluginapi.StorageUploadStep{}, err
		}
		o, e := a.get(target)
		if e != nil {
			return pluginapi.StorageUploadStep{}, e
		}
		if o.IsDir || o.Size != input.Size {
			return pluginapi.StorageUploadStep{}, fail("verification_failed", 409, "上传文件大小未通过核验")
		}
		entry, e := a.entry(target, o)
		return pluginapi.StorageUploadStep{Entry: &entry}, e
	}
	sha := ""
	for _, h := range input.Hashes {
		if h.Algorithm == "sha256" {
			sha = h.Value
		}
	}
	if sha == "" {
		return pluginapi.StorageUploadStep{HashAlgorithm: "sha256"}, nil
	}
	if err = a.absent(target); err != nil {
		return pluginapi.StorageUploadStep{}, err
	}
	checkpoint = uploadCheckpoint{Path: target, Size: input.Size, SHA256: sha, Namespace: a.namespace}
	b, _ := json.Marshal(checkpoint)
	return pluginapi.StorageUploadStep{Checkpoint: b, Request: &pluginapi.StorageHTTPRequest{URL: a.url("/api/fs/put"), Method: "PUT", Headers: map[string]string{"Authorization": a.creds.Token, "File-Path": url.PathEscape(target), "As-Task": "false", "Content-Type": "application/octet-stream"}, Offset: 0, Length: input.Size}}, nil
}
