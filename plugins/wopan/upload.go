package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopan"
)

const chunkSize int64 = 16 << 20

type uploadState struct {
	Phase  string `json:"phase"`
	Batch  string `json:"batch"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Parent string `json:"parent"`
	Size   int64  `json:"size"`
	Part   int64  `json:"part"`
	Total  int64  `json:"total"`
	Info   string `json:"info"`
	Access string `json:"access"`
	SHA256 string `json:"sha256"`
}

func (a *adapter) upload(ctx context.Context, p pluginapi.StorageParams) (pluginapi.StorageUploadStep, error) {
	var empty pluginapi.StorageUploadStep
	if err := validName(p.Name); err != nil {
		return empty, err
	}
	parent, err := decodeRef(p.ParentID)
	if err != nil || !parent.Dir {
		return empty, errors.New("请选择联通云盘目录")
	}
	in := p.Upload
	if in == nil || in.Size <= 0 {
		return empty, errors.New("联通云盘不支持上传空文件")
	}
	var s uploadState
	if len(in.Checkpoint) > 0 {
		if json.Unmarshal(in.Checkpoint, &s) != nil || s.ID == "" || s.Name != p.Name || s.Size != in.Size || s.Parent != parent.ID || s.Total != (in.Size-1)/chunkSize+1 || s.SHA256 == "" || (s.Phase != "quick" && s.Phase != "part") {
			return empty, errors.New("联通云盘上传检查点无效")
		}
		if s.Phase == "quick" {
			if in.Resuming || in.Response != nil {
				return empty, errors.New("联通云盘秒传回执未确认，请检查远端后重新创建上传任务")
			}
			if err = waitWrite(ctx); err != nil {
				return empty, err
			}
			fid, hit, err := a.client.QuickTransfer(ctx, s.SHA256, s.Name, s.Parent, s.Size, s.Batch)
			if err != nil {
				return empty, err
			}
			if hit {
				return a.uploaded(ctx, s.Parent, s.Name, fid, s.Size, true)
			}
			s.Info, err = a.client.UploadInfo(s.Name, s.Parent, s.Batch, s.Size)
			if err != nil {
				return empty, err
			}
			s.Access, _ = a.client.Tokens()
			s.Part = 1
			s.Phase = "part"
			return uploadPlan(s), nil
		}
		if s.Part < 1 || s.Part > s.Total || s.Info == "" || len(s.Access) < 16 {
			return empty, errors.New("联通云盘上传检查点无效")
		}
		// The supplied protocol has no list-parts endpoint. Never resend a part whose
		// receipt was lost, especially the final chunk which creates the remote file.
		if in.Response == nil {
			return empty, errors.New("联通云盘上传回执未确认，请检查远端后重新创建上传任务")
		}
		var response struct {
			Code string `json:"code"`
			Data struct {
				FID string `json:"fid"`
			} `json:"data"`
		}
		if in.Response.Status != 200 || json.Unmarshal([]byte(in.Response.Body), &response) != nil || response.Code != "0000" {
			return empty, errors.New("联通云盘上传分片未确认，请检查远端")
		}
		if s.Part == s.Total {
			if response.Data.FID == "" {
				return empty, errors.New("联通云盘上传结束未返回文件标识，请检查远端")
			}
			return a.uploaded(ctx, s.Parent, s.Name, response.Data.FID, s.Size, false)
		}
		s.Part++
	} else {
		if in.Response != nil {
			return empty, errors.New("联通云盘上传回执缺少检查点")
		}
		sha := ""
		for _, h := range in.Hashes {
			if strings.EqualFold(h.Algorithm, "sha256") {
				sha = strings.ToLower(h.Value)
			}
		}
		if sha == "" {
			return pluginapi.StorageUploadStep{HashAlgorithm: "sha256"}, nil
		}
		digest, err := hex.DecodeString(sha)
		if err != nil || len(digest) != 32 {
			return empty, errors.New("联通云盘上传 SHA256 无效")
		}
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return empty, err
		}
		batch := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("20060102150405")
		s = uploadState{Phase: "quick", Batch: batch, ID: fmt.Sprintf("%d_%s", time.Now().UnixMilli(), hex.EncodeToString(nonce[:])), Name: p.Name, Parent: parent.ID, Size: in.Size, Total: (in.Size-1)/chunkSize + 1, SHA256: sha}
		checkpoint, _ := json.Marshal(s)
		return pluginapi.StorageUploadStep{SaveCheckpoint: true, Checkpoint: checkpoint}, nil
	}
	return uploadPlan(s), nil
}
func uploadPlan(s uploadState) pluginapi.StorageUploadStep {
	offset := (s.Part - 1) * chunkSize
	length := chunkSize
	if length > s.Size-offset {
		length = s.Size - offset
	}
	fields := []pluginapi.StorageFormField{{Name: "accessToken", Value: s.Access}, {Name: "uniqueId", Value: s.ID}, {Name: "psToken", Value: ""}, {Name: "fileName", Value: s.Name}, {Name: "fileSize", Value: strconv.FormatInt(s.Size, 10)}, {Name: "totalPart", Value: strconv.FormatInt(s.Total, 10)}, {Name: "partSize", Value: strconv.FormatInt(length, 10)}, {Name: "partIndex", Value: strconv.FormatInt(s.Part, 10)}, {Name: "channel", Value: "wocloud"}, {Name: "directoryId", Value: s.Parent}, {Name: "fileInfo", Value: s.Info}}
	checkpoint, _ := json.Marshal(s)
	return pluginapi.StorageUploadStep{Checkpoint: checkpoint, Request: &pluginapi.StorageHTTPRequest{URL: wopan.DefaultUploadHost + "/openapi/client/upload2C", Method: "POST", Offset: offset, Length: length, Headers: map[string]string{"User-Agent": wopan.DefaultUserAgent, "Origin": "https://pan.wo.cn", "Referer": "https://pan.wo.cn/", "Access-Token": s.Access, "X-YP-Access-Token": s.Access}, Multipart: &pluginapi.StorageMultipart{FileField: "file", FileName: s.Name, Fields: fields}}}
}
func (a *adapter) uploaded(ctx context.Context, parent, name, fid string, size int64, reused bool) (pluginapi.StorageUploadStep, error) {
	// Completion needs the list ID and FID, which are different identifiers.
	e, err := a.find(ctx, parent, func(e wopan.Entry) bool {
		return !e.IsDir && (e.FID == fid || e.ID == fid) && e.Name == name && e.Size == size
	})
	if err != nil {
		return pluginapi.StorageUploadStep{}, errors.New("联通云盘已接收文件，目录尚未确认，请检查远端，勿重复上传")
	}
	parentRef, decodeErr := decodeRef(a.call.Params.ParentID)
	if decodeErr != nil {
		return pluginapi.StorageUploadStep{}, decodeErr
	}
	result := entry(e, parentRef)
	return pluginapi.StorageUploadStep{Entry: &result, Reused: reused}, nil
}
