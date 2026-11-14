package wopan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	SpacePersonal = "0"
	SpaceFamily   = "1"

	partSize   = 16 << 20 // 与客户端 CHUNK_SIZE 一致
	maxRetries = 3
)

type Entry struct {
	ID          string `json:"id"`
	FID         string `json:"fid"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	IsDir       bool   `json:"is_dir"`
	ModTime     string `json:"modified"`
	FileVersion string `json:"file_version,omitempty"`
	SpaceType   string `json:"space_type,omitempty"`
}

type Storage struct {
	UsedSpace int64 `json:"used_space"`
	AllSpace  int64 `json:"all_space"`
}

// List 列出目录；自动翻页，避免 >100 项漏文件
func (c *Client) List(ctx context.Context, spaceType, dirID, familyID string) ([]Entry, error) {
	const size = 500
	out := []Entry{}
	for page := 0; page < 1000; page++ {
		params := map[string]any{
			"spaceType":         spaceType,
			"parentDirectoryId": dirID,
			"pageNum":           page,
			"pageSize":          size,
			"sortRule":          "0",
		}
		if familyID != "" && spaceType == SpaceFamily {
			params["familyId"] = familyID
		}
		if dn := c.DeviceNo(); dn != "" {
			params["deviceNo"] = dn
		}
		data, err := c.dispatcher(ctx, "wohome", "QueryAllFiles", params, true)
		if err != nil {
			return nil, err
		}
		var r struct {
			Files []struct {
				ID          string `json:"id"`
				FID         string `json:"fid"`
				Name        string `json:"name"`
				Type        int    `json:"type"`
				Size        int64  `json:"size"`
				CreateTime  string `json:"createTime"`
				UpdateTime  string `json:"updateTime"`
				FileVersion any    `json:"fileVersion"`
				SpaceType   any    `json:"spaceType"`
			} `json:"files"`
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("wopan: 目录响应解析失败: %w", err)
		}
		for _, f := range r.Files {
			mod := f.UpdateTime
			if mod == "" {
				mod = f.CreateTime
			}
			out = append(out, Entry{
				ID: f.ID, FID: f.FID, Name: f.Name, Size: f.Size,
				IsDir: f.Type == 0, ModTime: mod, FileVersion: toString(f.FileVersion),
				SpaceType: toString(f.SpaceType),
			})
		}
		if len(r.Files) < size {
			break
		}
	}
	return out, nil
}

// explainOp 把服务端那几句没营养的错误（9999/500/1000）翻成能照着改的提示。
// 触发场景（实测）：把目录 id 当文件传、is_dir 传错、fid 为空或没 URL 编码。
func explainOp(op string, err error) error {
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Code {
		case "500010006":
			return fmt.Errorf("wopan: %s失败：文件被占用（服务端异步任务未完成），请稍后重试", op)
		case "9999":
			return fmt.Errorf("wopan: %s失败（9999 系统异常）—— 常见原因：写操作太频繁被限流（放慢重试）；或 id 类型不匹配", op)
		case "500", "1000":
			return fmt.Errorf("wopan: %s失败（%s %s）—— 常见原因：id 类型不匹配（目录 id 当文件用 / is_dir 传错）、fid 为空或未 URL 编码",
				op, ae.Code, ae.Desc)
		}
	}
	return err
}

// DownloadURL 取直链（保持服务端原样，不改写域名）
func (c *Client) DownloadURL(ctx context.Context, fid, spaceType string) (string, error) {
	if fid == "" {
		return "", errors.New("wopan: fid 为空（目录没有 fid；列表里的 fid 可能为空，请用文件项）")
	}
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	data, err := c.dispatcher(ctx, "wohome", "GetDownloadUrlV2", map[string]any{
		"type": "1", "fidList": []string{fid}, "spaceType": spaceType,
	}, true)
	if err != nil {
		return "", err
	}
	var r struct {
		List []struct {
			DownloadURL string `json:"downloadUrl"`
			URL         string `json:"url"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("wopan: 下载链接响应解析失败: %w", err)
	}
	if len(r.List) == 0 {
		return "", errors.New("wopan: 响应里没有下载链接")
	}
	if r.List[0].DownloadURL != "" {
		return r.List[0].DownloadURL, nil
	}
	return r.List[0].URL, nil
}

// Mkdir 创建目录，返回新目录 id（"130007" = 已存在，也视为成功）
func (c *Client) Mkdir(ctx context.Context, spaceType, parentID, name, familyID string) (string, error) {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	params := map[string]any{
		"spaceType":         spaceType,
		"parentDirectoryId": parentID,
		"directoryName":     name,
		"clientType":        1,
	}
	if familyID != "" {
		params["familyId"] = familyID
	}
	if dn := c.DeviceNo(); dn != "" {
		params["deviceNo"] = dn
	}
	if err := c.throttleWrite(ctx); err != nil {
		return "", err
	}
	data, err := c.dispatcher(ctx, "wohome", "CreateDirectory", params, true)
	if err != nil {
		var ae *APIError
		if errors.As(err, &ae) && (ae.Code == "130007" || ae.Code == "200") {
			// 目录已存在：调用方需要 id，回查父目录拿同名目录
			if list, lerr := c.List(ctx, spaceType, parentID, familyID); lerr == nil {
				for _, e := range list {
					if e.IsDir && e.Name == name {
						return e.ID, nil
					}
				}
			}
			return "", errors.New("wopan: 同名目录未确认，请检查远端")
		}
		// An unknown receipt must not replay a write or report success.
		return "", explainOp("创建目录", err)
	}
	var r struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &r) != nil || r.ID == "" {
		return "", errors.New("wopan: 创建目录未返回标识，请检查远端")
	}
	return r.ID, nil
}

// Rename 重命名；isDir 决定服务端的 type（目录 0 / 文件 1）。
// fileVersion 仅对文件生效（客户端 `2==target_type && (h.fileVersion=p)`），冲突/覆盖时服务端会校它；
// 返回新版本号（客户端重命名后用 DATA 里的新 version 更新本地缓存）。
func (c *Client) Rename(ctx context.Context, spaceType, id, name string, isDir bool, fileType, familyID, fileVersion string) (string, error) {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	t := 1
	if isDir {
		t = 0
	}
	params := map[string]any{
		"spaceType":  spaceType,
		"type":       t,
		"id":         id,
		"name":       name,
		"clientType": 1,
	}
	if fileType != "" {
		params["fileType"] = fileType
	}
	if familyID != "" {
		params["familyId"] = familyID
	}
	if !isDir && fileVersion != "" {
		params["fileVersion"] = fileVersion
	}
	if dn := c.DeviceNo(); dn != "" {
		params["deviceNo"] = dn
	}
	if err := c.throttleWrite(ctx); err != nil {
		return "", err
	}
	data, err := c.dispatcher(ctx, "wohome", "RenameFileOrDirectory", params, true)
	if err != nil {
		return "", explainOp("重命名", err)
	}
	debugRaw("rename", data)
	// 服务端 DATA 可能是标量新版本号（实测就是 `2`），也可能是对象；两种都兼容
	if s := strings.TrimSpace(string(data)); s != "" && s != "null" {
		var obj struct {
			FileVersion any `json:"fileVersion"`
			Version     any `json:"version"`
		}
		if json.Unmarshal(data, &obj) == nil {
			if v := toString(obj.FileVersion); v != "" {
				return v, nil
			}
			if v := toString(obj.Version); v != "" {
				return v, nil
			}
		}
		var scalar any
		if json.Unmarshal(data, &scalar) == nil {
			return toString(scalar), nil
		}
	}
	return "", nil
}

// BatchTask 删除/移动/复制统一走 /wohome/pass/file/batch/createTask
type batchReq struct {
	TaskType           string   `json:"taskType"`
	SourceSpaceType    string   `json:"sourceSpaceType"`
	TargetSpaceType    string   `json:"targetSpaceType,omitempty"`
	SourceFileIds      []string `json:"sourceFileIds"`
	SourceDirectoryIds []string `json:"sourceDirectoryIds"`
	TargetDirectoryID  string   `json:"targetDirectoryId,omitempty"`
	TargetFamilyID     string   `json:"targetFamilyId,omitempty"`
	SourceFamilyID     string   `json:"sourceFamilyId,omitempty"`
	DeviceNo           string   `json:"deviceNo,omitempty"`
	NotEmptyDir        *int     `json:"notEmptyDir,omitempty"`
	FileVersion        string   `json:"fileVersion,omitempty"` // 仅文件、仅移动时带
}

func (c *Client) batch(ctx context.Context, req batchReq) error {
	if req.SourceFileIds == nil {
		req.SourceFileIds = []string{}
	}
	if req.SourceDirectoryIds == nil {
		req.SourceDirectoryIds = []string{}
	}
	req.DeviceNo = c.DeviceNo()

	// 刚上传/移动完的文件会被服务端异步任务短暂占用，删除时回 500010006。
	// 客户端也是重试的（3s*5^n），这里用 4 次小退避，够覆盖实测的占用窗口。
	delay := 300 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if e := c.throttleWrite(ctx); e != nil {
			return e
		}
		data, err := c.restJSON(ctx, http.MethodPost, "/wohome/pass/file/batch/createTask", req, true)
		if err != nil {
			lastErr = err
			var ae *APIError
			if errors.As(err, &ae) && ae.Code == "500010006" {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
				delay *= 2
				continue
			}
			return err
		}
		var r struct {
			Success     *bool  `json:"success"`
			Finish      bool   `json:"finish"`
			FailMessage string `json:"failMessage"`
		}
		if json.Unmarshal(data, &r) != nil || r.Success == nil {
			return errors.New("联通云盘写入回执未确认，请检查远端后重试")
		}
		if !*r.Success {
			if r.FailMessage == "" {
				r.FailMessage = "服务端返回 success=false"
			}
			return errors.New("wopan: " + r.FailMessage)
		}
		return nil
	}
	var ae *APIError
	if errors.As(lastErr, &ae) && ae.Code == "500010006" {
		return fmt.Errorf("wopan: 文件被占用（服务端异步任务未完成），已重试 4 次仍失败，请稍后再试")
	}
	if errors.As(lastErr, &ae) && ae.Code == "9999" {
		return fmt.Errorf("wopan: 服务端系统异常（9999），多次重试仍失败 —— 大概率是写操作太频繁触发限流，请放慢或稍后再试")
	}
	return lastErr
}

// Move 移动文件/目录；单文件移动时带 fileVersion（与客户端一致，服务端靠它判冲突）
func (c *Client) Move(ctx context.Context, spaceType string, fileIDs, dirIDs []string, targetDirID, sourceFamilyID, targetFamilyID, fileVersion string) error {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	req := batchReq{
		TaskType: "MOVE", SourceSpaceType: spaceType, TargetSpaceType: spaceType,
		SourceFileIds: fileIDs, SourceDirectoryIds: dirIDs,
		TargetDirectoryID: targetDirID, SourceFamilyID: sourceFamilyID, TargetFamilyID: targetFamilyID,
	}
	if len(fileIDs) == 1 && len(dirIDs) == 0 && fileVersion != "" {
		req.FileVersion = fileVersion
	}
	return explainOp("移动", c.batch(ctx, req))
}

func (c *Client) Copy(ctx context.Context, spaceType string, fileIDs, dirIDs []string, targetDirID, sourceFamilyID, targetFamilyID string) error {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	return explainOp("复制", c.batch(ctx, batchReq{
		TaskType: "COPY", SourceSpaceType: spaceType, TargetSpaceType: spaceType,
		SourceFileIds: fileIDs, SourceDirectoryIds: dirIDs,
		TargetDirectoryID: targetDirID, SourceFamilyID: sourceFamilyID, TargetFamilyID: targetFamilyID,
	}))
}

// Delete 删除（进回收站）。目录非空时服务端直接拒（5056），所以先递归清子项再删目录。
func (c *Client) Delete(ctx context.Context, spaceType string, fileIDs, dirIDs []string) error {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	if len(fileIDs) > 0 {
		if err := c.batch(ctx, batchReq{TaskType: "DELETE", SourceSpaceType: spaceType, SourceFileIds: fileIDs}); err != nil {
			return explainOp("删除文件", err)
		}
	}
	for _, d := range dirIDs {
		if err := c.deleteDirTree(ctx, spaceType, d, 0); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) deleteDirTree(ctx context.Context, spaceType, dirID string, depth int) error {
	if depth > 32 {
		return errors.New("wopan: 目录层数过深，放弃递归删除")
	}
	entries, err := c.List(ctx, spaceType, dirID, "")
	if err != nil {
		return err
	}
	var files, subs []string
	for _, e := range entries {
		if e.IsDir {
			subs = append(subs, e.ID)
		} else {
			files = append(files, e.ID)
		}
	}
	if len(files) > 0 {
		if err := c.batch(ctx, batchReq{TaskType: "DELETE", SourceSpaceType: spaceType, SourceFileIds: files}); err != nil {
			return err
		}
	}
	for _, sd := range subs {
		if err := c.deleteDirTree(ctx, spaceType, sd, depth+1); err != nil {
			return err
		}
	}
	// 服务端删的是异步任务，父目录必须等子项真正没了才能删
	if len(entries) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return c.batch(ctx, batchReq{TaskType: "DELETE", SourceSpaceType: spaceType, SourceDirectoryIds: []string{dirID}})
}

// EmptyRecycle 清空回收站
func (c *Client) EmptyRecycle(ctx context.Context) error {
	_, err := c.dispatcher(ctx, "wohome", "EmptyRecycleData", map[string]any{}, true)
	return err
}

// Storage 容量：POST /api/bff/member/info/card/storage（该接口未登录返回 10002404，已统一在 restJSON 里续期）
func (c *Client) Storage(ctx context.Context) (Storage, error) {
	const path = "/api/bff/member/info/card/storage"
	data, err := c.restJSON(ctx, http.MethodPost, path, map[string]any{}, true)
	if err != nil {
		if errors.Is(err, ErrNeedLogin) { // 别用 GET 回退把鉴权错误盖成 500
			return Storage{}, err
		}
		data, err = c.restJSON(ctx, http.MethodGet, path, nil, true)
	}
	if err != nil {
		return Storage{}, err
	}
	var r struct {
		UsedSpace any `json:"usedSpace"`
		AllSpace  any `json:"allSpace"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Storage{}, fmt.Errorf("wopan: 容量响应解析失败: %w", err)
	}
	return Storage{UsedSpace: toInt(r.UsedSpace), AllSpace: toInt(r.AllSpace)}, nil
}

// UploadOptions 上传参数
type UploadOptions struct {
	LocalPath string
	Name      string // 服务端看到的文件名；空则用 LocalPath 的 basename
	DirID     string
	SpaceType string
	FamilyID  string
	Progress  func(done, total int64)
}

// UploadFile 先试秒传，再走 16MiB 分片 upload2C；返回 fid
func (c *Client) UploadFile(ctx context.Context, opt UploadOptions) (string, error) {
	if opt.SpaceType == "" {
		opt.SpaceType = SpacePersonal
	}
	st, err := os.Stat(opt.LocalPath)
	if err != nil {
		return "", err
	}
	size := st.Size()
	name := opt.Name
	if name == "" {
		name = filepath.Base(opt.LocalPath)
	}
	if err := c.throttleWrite(ctx); err != nil {
		return "", err
	}
	if size == 0 {
		// 客户端也不收空文件（Uppy 层按 size>0 过滤），服务端对 0 字节分片回 400
		return "", errors.New("wopan: 空文件（0 字节）不支持上传")
	}
	batchNo := time.Now().Format("20060102150405")
	uniqueID := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), randSuffix(6))

	if fid, ok, qerr := c.tryQuickTransfer(ctx, opt, name, size, batchNo); qerr != nil {
		return "", qerr
	} else if ok {
		if opt.Progress != nil {
			opt.Progress(size, size)
		}
		return fid, nil
	}

	total := int((size + partSize - 1) / partSize)
	if total == 0 {
		total = 1
	}
	f, err := os.Open(opt.LocalPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fileInfoKey, _ := c.Tokens()
	fileInfo := map[string]any{
		"spaceType":   opt.SpaceType,
		"directoryId": opt.DirID,
		"batchNo":     batchNo,
		"fileName":    name,
		"fileSize":    size,
		"fileType":    fileTypeOf(name),
		"deviceNo":    c.DeviceNo(),
		"clientType":  1,
	}
	if opt.FamilyID != "" {
		fileInfo["familyId"] = opt.FamilyID
	}
	fiJSON, _ := json.Marshal(fileInfo)
	encFileInfo, err := aesEncrypt(string(fiJSON), fileInfoKey)
	if err != nil {
		return "", err
	}

	var lastFID string
	var done int64
	// ponytail: 顺序上传，3 次重试；要并发就开 3 个 goroutine，按 MAX_CONCURRENT_CHUNKS
	for idx := 1; idx <= total; idx++ {
		off := int64(idx-1) * partSize
		n := int64(partSize)
		if off+n > size {
			n = size - off
		}
		fields := [][2]string{
			{"accessToken", fileInfoKey},
			{"uniqueId", uniqueID},
			{"psToken", ""},
			{"fileName", name},
			{"fileSize", strconv.FormatInt(size, 10)},
			{"totalPart", strconv.Itoa(total)},
			{"partSize", strconv.FormatInt(n, 10)},
			{"partIndex", strconv.Itoa(idx)},
			{"channel", "wocloud"},
			{"directoryId", opt.DirID},
			{"fileInfo", encFileInfo},
		}
		var fid string
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(attempt) * 2 * time.Second):
				}
			}
			fid, lastErr = c.uploadPart(ctx, f, off, n, fields, name)
			if lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			return "", fmt.Errorf("wopan: 第 %d/%d 片上传失败: %w", idx, total, lastErr)
		}
		if fid != "" {
			lastFID = fid
		}
		done += n
		if opt.Progress != nil {
			opt.Progress(done, size)
		}
	}
	if lastFID == "" {
		return "", errors.New("wopan: 上传完成但服务端未返回 fid")
	}
	return lastFID, nil
}

func (c *Client) uploadPart(ctx context.Context, f *os.File, off, n int64, fields [][2]string, fileName string) (string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	go func() {
		var gerr error
		defer func() {
			_ = mw.Close()
			if gerr != nil {
				_ = pw.CloseWithError(gerr)
				return
			}
			_ = pw.Close()
		}()
		for _, kv := range fields {
			if gerr = mw.WriteField(kv[0], kv[1]); gerr != nil {
				return
			}
		}
		var fw io.Writer
		if fw, gerr = mw.CreateFormFile("file", fileName); gerr != nil {
			return
		}
		_, gerr = io.CopyN(fw, io.NewSectionReader(f, off, n), n)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.UploadHost+"/openapi/client/upload2C", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Origin", "https://pan.wo.cn")
	req.Header.Set("Referer", "https://pan.wo.cn/")
	tok, _ := c.Tokens()
	req.Header.Set("Access-Token", tok)
	req.Header.Set("X-YP-Access-Token", tok)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wopan: 上传分片 HTTP %d: %s", resp.StatusCode, truncate(raw, 200))
	}
	var r struct {
		Code string `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			FID string `json:"fid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("wopan: 上传分片响应解析失败: %s", truncate(raw, 200))
	}
	if r.Code != "0000" {
		return "", &APIError{Code: r.Code, Desc: r.Msg}
	}
	return r.Data.FID, nil
}

// tryQuickTransfer 秒传：整文件 sha256（小写 hex，与客户端 getFullFileHash 一致）命中即完成。
// 返回 hit=false/err=nil 表示“未命中，请正常上传”；err 非 nil 表示不要再传（如配额不足）。
func (c *Client) tryQuickTransfer(ctx context.Context, opt UploadOptions, name string, size int64, batchNo string) (string, bool, error) {
	f, err := os.Open(opt.LocalPath)
	if err != nil {
		return "", false, nil // 读不到就不秒传，交给上传环节报错
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false, nil
	}
	return c.quickTransferOnce(ctx, hex.EncodeToString(h.Sum(nil)), name, size, batchNo, opt)
}

func (c *Client) quickTransferOnce(ctx context.Context, sha, name string, size int64, batchNo string, opt UploadOptions) (string, bool, error) {
	req := map[string]any{
		"sha256": sha, "fileSize": size, "fileName": name, "fileType": fileTypeOf(name),
		"batchNo": batchNo, "spaceType": opt.SpaceType, "directoryId": opt.DirID,
		"fileModificationTime": time.Now().UnixMilli(),
	}
	if opt.FamilyID != "" {
		req["familyId"] = opt.FamilyID
	}
	b, _ := json.Marshal(req)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, quickTransferAPI, bytes.NewReader(b))
	if err != nil {
		return "", false, nil
	}
	tok, _ := c.Tokens()
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("User-Agent", c.cfg.UserAgent)
	// 客户端用的就是小写 access-token（HTTP 头不分大小写，无妨）
	hr.Header.Set("access-token", tok)
	hr.Header.Set("X-YP-Access-Token", tok)
	resp, err := c.http.Do(hr)
	if err != nil {
		return "", false, nil // 探测失败不影响上传
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Meta struct {
			Code string `json:"code"`
		} `json:"meta"`
		Result struct {
			HasFile any    `json:"hasFile"`
			FID     string `json:"fid"`
			FileID  string `json:"fileId"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return "", false, nil
	}
	// 2001 = 配额不足。客户端此时直接失败，不再传任何字节；不拦就是白传一遍再被拒。
	if r.Meta.Code == "2001" {
		log.Printf("[wopan] 秒传探测返回 2001（容量不足），中止上传: %s", name)
		return "", false, &APIError{Code: "2001", Desc: "云盘容量不足"}
	}
	if r.Meta.Code != "0000" || toInt(r.Result.HasFile) != 1 {
		log.Printf("[wopan] 秒传未命中: %s code=%s hasFile=%v", name, r.Meta.Code, r.Result.HasFile)
		return "", false, nil
	}
	// 命中：服务端已建记录，0 字节上传
	if r.Result.FID != "" {
		log.Printf("[wopan] 秒传命中: %s (%d bytes) fid=%s", name, size, r.Result.FID)
		return r.Result.FID, true, nil
	}
	if r.Result.FileID != "" {
		log.Printf("[wopan] 秒传命中: %s (%d bytes) fileId=%s", name, size, r.Result.FileID)
		return r.Result.FileID, true, nil
	}
	log.Printf("[wopan] 秒传命中但没返回 id: %s", name)
	return "", true, nil
}

// debugRaw 仅在 WOPAN_DEBUG=1 时打印服务端原始返回，便于校对字段名
func debugRaw(tag string, b []byte) {
	if os.Getenv("WOPAN_DEBUG") != "1" {
		return
	}
	s := string(b)
	if len(s) > 700 {
		s = s[:700] + "…"
	}
	log.Printf("[wopan] raw(%s): %s", tag, s)
}

// parseFiles 解析文件列表；注意两种通道字段不统一（dispatcher 给 size 数字，pc 给 fileSize 字符串）
func (c *Client) parseFiles(raw json.RawMessage) ([]Entry, bool, error) {
	var r struct {
		Files []struct {
			ID          string `json:"id"`
			FID         string `json:"fid"`
			Name        string `json:"name"`
			FileName    string `json:"fileName"`
			Type        int    `json:"type"`
			Size        any    `json:"size"`
			FileSize    any    `json:"fileSize"`
			CreateTime  string `json:"createTime"`
			UpdateTime  string `json:"updateTime"`
			FileVersion any    `json:"fileVersion"`
			SpaceType   any    `json:"spaceType"`
		} `json:"files"`
		HasNext bool `json:"hasNext"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, false, fmt.Errorf("wopan: 文件列表解析失败: %w", err)
	}
	out := make([]Entry, 0, len(r.Files))
	for _, f := range r.Files {
		mod := f.UpdateTime
		if mod == "" {
			mod = f.CreateTime
		}
		name := f.Name
		if name == "" {
			name = f.FileName
		}
		sz := f.Size
		if sz == nil {
			sz = f.FileSize
		}
		out = append(out, Entry{
			ID: f.ID, FID: f.FID, Name: name, Size: toInt(sz),
			IsDir: f.Type == 0, ModTime: mod,
			FileVersion: toString(f.FileVersion), SpaceType: toString(f.SpaceType),
		})
	}
	return out, r.HasNext, nil
}

// Search 走 /wohome/pc/queryFilesByName（明文 REST，不需要加密）
// 注：/wohome/pc/queryFilesByDir 的 fileId 是“某个文件 id”（返回同目录列表），不是目录列表接口，故不用它。
func (c *Client) Search(ctx context.Context, keyword, spaceType, familyID string, page, size int) ([]Entry, bool, error) {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	if size <= 0 {
		size = 100
	}
	body := map[string]any{
		"keyword":   keyword,
		"spaceType": spaceType,
		"pageNum":   page,
		"pageSize":  size,
	}
	if familyID != "" && spaceType == SpaceFamily {
		body["familyId"] = familyID
	}
	data, err := c.restJSON(ctx, http.MethodPost, "/wohome/pc/queryFilesByName", body, true)
	if err != nil {
		// 该接口实测偶发 504（openresty/APISIX 网关超时），读接口重试一次就好
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
		if data, err = c.restJSON(ctx, http.MethodPost, "/wohome/pc/queryFilesByName", body, true); err != nil {
			return nil, false, err
		}
	}
	debugRaw("search", data)
	return c.parseFiles(data)
}

// PathNode 目录路径上的一级（面包屑用）
type PathNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Dir  string `json:"directoryName"`
}

// DirPath 取目录的祖先链（dispatcher key: GetDirectoryPath）。
// 服务端返回「深→浅」（当前目录在前），这里反转成「浅→深」方便拼路径。
func (c *Client) DirPath(ctx context.Context, spaceType, dirID, familyID string) ([]PathNode, error) {
	if spaceType == "" {
		spaceType = SpacePersonal
	}
	params := map[string]any{"spaceType": spaceType, "directoryId": dirID}
	if familyID != "" {
		params["familyId"] = familyID
	}
	data, err := c.dispatcher(ctx, "wohome", "GetDirectoryPath", params, true)
	if err != nil {
		return nil, err
	}
	debugRaw("dirpath", data)
	var arr []PathNode
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, fmt.Errorf("wopan: 目录路径解析失败: %w", err)
	}
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
	for i := range arr {
		if arr[i].Name == "" {
			arr[i].Name = arr[i].Dir
		}
	}
	return arr, nil
}

// Download 打开文件流（直链可能带签名，不做任何改写）
func (c *Client) Download(ctx context.Context, fid, spaceType string) (io.ReadCloser, int64, error) {
	u, err := c.DownloadURL(ctx, fid, spaceType)
	if err != nil {
		return nil, 0, err
	}
	if _, err := url.Parse(u); err != nil {
		return nil, 0, fmt.Errorf("wopan: 下载链接非法: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("wopan: 下载 HTTP %d", resp.StatusCode)
	}
	return resp.Body, resp.ContentLength, nil
}

func randSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// fileTypeOf 按扩展名映射客户端的 0~5：0 其它/未知、1 图片、2 视频、3 音频、4 文档、5 xmind/other。
// ponytail: 未识别的扩展名一律归 0（客户端的分类表更细），误差只影响相册/视频分类展示。
func fileTypeOf(name string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")) {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "heic", "heif", "svg", "ico", "tif", "tiff", "raw":
		return "1"
	case "mp4", "mkv", "avi", "mov", "flv", "wmv", "mpeg", "mpg", "webm", "m4v", "3gp", "ts", "rmvb":
		return "2"
	case "mp3", "wav", "flac", "aac", "ogg", "m4a", "wma", "ape", "opus":
		return "3"
	case "doc", "docx", "xls", "xlsx", "ppt", "pptx", "pdf", "txt", "md", "rtf", "csv", "log", "json", "xml", "html", "htm", "epub":
		return "4"
	case "xmind":
		return "5"
	}
	return "0"
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func toInt(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}
