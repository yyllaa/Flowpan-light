package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flowpan/app/pluginapi"
	"fmt"
	"math"
	"strings"
	"time"
)

func objectInput(value any) (map[string]any, error) {
	if value == nil || value == "" {
		return map[string]any{}, nil
	}
	if text, ok := value.(string); ok {
		var v map[string]any
		if json.Unmarshal([]byte(text), &v) != nil || v == nil {
			return nil, fmt.Errorf("请输入 JSON 对象")
		}
		return v, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil || v == nil {
		return nil, fmt.Errorf("请输入 JSON 对象")
	}
	return v, nil
}

func executeTool(h host, r pluginapi.Request) (pluginapi.Result, error) {
	if r.TaskID == "" {
		return pluginapi.Result{}, fmt.Errorf("缺少后台任务编号")
	}
	action := r.ActionID
	op, ok := map[string]string{"profile": "me", "avatar": "avatar", "query": "list-shares", "virtual-cache": "virtual-cache", "sha1-query": "tmdb-sha1", "share-check": "check-sharecode", "rss-options": "rss-options", "rss": "rss", "share": "share", "developer-usage": "developer-usage"}[action]
	if !ok {
		return pluginapi.Result{}, fmt.Errorf("插件动作不存在")
	}
	if action == "share" && !r.Confirmed {
		return pluginapi.Result{}, fmt.Errorf("请先确认发布分享")
	}
	input := map[string]any{}
	for k, v := range r.Input {
		if v != nil && v != "" {
			input[k] = v
		}
	}
	delete(input, "refresh")
	var err error
	if (action == "share" || action == "developer-usage") && input["payload"] != nil {
		input, err = objectInput(input["payload"])
	}
	if err != nil {
		return pluginapi.Result{}, err
	}
	if action == "query" {
		filters, e := objectInput(input["filters"])
		if e != nil {
			return pluginapi.Result{}, e
		}
		delete(input, "filters")
		for k, v := range filters {
			if k == "tmdb_id" || k == "media_type" || k == "resource_id" {
				return pluginapi.Result{}, fmt.Errorf("请在作品栏填写作品编号")
			}
			input[k] = v
		}
		if err := normalizeDianQuery(input); err != nil {
			return pluginapi.Result{}, err
		}
	}
	if err := normalizeExtension(action, input); err != nil {
		return pluginapi.Result{}, err
	}
	client := dianHostClient{h: h}
	var identity pluginapi.SDKResponse
	if err := h.Call("sdk.call", pluginapi.SDKRequest{Provider: "dian115", Operation: "identity"}, &identity); err != nil {
		return pluginapi.Result{}, err
	}
	if identity.Account == "" {
		return pluginapi.Result{}, fmt.Errorf("请配置癫影 OpenAPI Key")
	}
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	cacheHash := sha256.Sum256([]byte(identity.Account + "\x00" + action + "\x00" + string(encoded)))
	key := "dian-tool-" + hex.EncodeToString(cacheHash[:20])
	// The developer endpoint is site-limited per key, regardless of the filters.
	if action == "developer-usage" {
		key = identity.Account + "-developer-usage"
	}
	now := time.Now().Unix()
	var previous receipt
	if err := h.Call("kv.get", map[string]any{"key": key}, &previous); err != nil {
		return pluginapi.Result{}, err
	}
	if previous.Pending {
		return pluginapi.Result{}, fmt.Errorf("上次发布结果待核验，请先在站点检查，避免重复提交")
	}
	refresh, _ := r.Input["refresh"].(bool)
	if previous.Until > now && !(action == "query" && refresh) {
		if action == "developer-usage" && previous.Result.Data["query_hash"] != hex.EncodeToString(sum[:12]) {
			return pluginapi.Result{}, fmt.Errorf("开发者用量每 12 小时查询一次，请稍后查询其他条件")
		}
		cached := previous.Result
		if response, ok := cached.Data["response"].(map[string]any); ok {
			cached.Items = nil
			cached.Table = nil
			extensionResult(&cached, action, response, input)
		}
		return cached, nil
	}
	if action == "share" {
		if err := h.Call("kv.set", map[string]any{"key": key, "value": receipt{Pending: true}}, nil); err != nil {
			return pluginapi.Result{}, err
		}
	}
	_ = h.Call("task.progress", pluginapi.Progress{Message: "通过 OpenAPI 读取" + action}, nil)
	if action == "avatar" && input["avatar_url"] == nil {
		me, e := client.GetUserInfo()
		if e != nil {
			return pluginapi.Result{}, e
		}
		if strings.TrimSpace(me.AvatarURL) == "" {
			result := pluginapi.Result{Message: "账号未设置头像", Data: map[string]any{"action_id": action}}
			if err := h.Call("kv.set", map[string]any{"key": key, "value": receipt{Until: now + 300, Result: result}}, nil); err != nil {
				return pluginapi.Result{}, err
			}
			_ = h.Call("task.progress", pluginapi.Progress{Done: 1, Total: 1, Percent: 100, Message: result.Message}, nil)
			return result, nil
		}
		input["avatar_url"] = me.AvatarURL
	}
	var raw json.RawMessage
	if err := client.call(op, input, &raw); err != nil {
		if writeRejected(err) && action == "share" {
			if e := h.Call("kv.set", map[string]any{"key": key, "value": receipt{}}, nil); e != nil {
				return pluginapi.Result{}, e
			}
		}
		return pluginapi.Result{}, err
	}
	var response map[string]any
	if json.Unmarshal(raw, &response) != nil {
		return pluginapi.Result{}, fmt.Errorf("OpenAPI 响应格式无效")
	}
	label := map[string]string{"profile": "账号资料已读取", "avatar": "头像已读取", "query": "资源查询完成", "virtual-cache": "虚拟缓存查询完成", "sha1-query": "SHA1 查询完成", "share-check": "分享链接查询完成", "rss-options": "RSS 筛选选项已读取", "rss": "RSS 更新已读取", "share": "分享已发布", "developer-usage": "开发者用量已读取"}[action]
	data := map[string]any{"action_id": action, "response": response, "query_hash": hex.EncodeToString(sum[:12])}
	if action == "query" {
		items, count, e := parseDianOpenAPIList(raw, fmt.Sprint(input["media_type"]))
		if e != nil {
			return pluginapi.Result{}, e
		}
		data["resources"] = resourceResult(items)["resources"]
		page, size := int(input["page"].(float64)), int(input["size"].(float64))
		hasMore := count >= size
		if more, ok := response["has_more"].(bool); ok {
			hasMore = more
		} else if total, ok := response["total"].(float64); ok && total >= 0 {
			hasMore = float64(page*size) < total
		}
		data["pagination"] = map[string]any{"page": page, "size": size, "has_more": hasMore}
	}
	if action == "avatar" {
		mime, _ := response["content_type"].(string)
		body, _ := response["data_base64"].(string)
		if strings.HasPrefix(mime, "image/") {
			data["avatar_url"] = "data:" + mime + ";base64," + body
		}
	}
	result := pluginapi.Result{Message: label, Data: data}
	extensionResult(&result, action, response, input)
	ttl := int64(300)
	if action == "share" {
		ttl = 30 * 86400
	}
	if action == "developer-usage" {
		ttl = 12 * 3600
	}
	b, _ := json.Marshal(result)
	cacheResult := result
	if action == "query" && len(b) >= 44<<10 {
		// A cache hit must retain the usable resource page, not just a receipt.
		cacheData := map[string]any{"action_id": action, "query_hash": data["query_hash"], "resources": data["resources"], "pagination": data["pagination"]}
		cacheResult = pluginapi.Result{Message: label, Data: cacheData}
		b, _ = json.Marshal(cacheResult)
	}
	if action == "query" && len(b) >= 44<<10 {
		// Oversized pages are returned intact without exceeding the KV limit.
		if err := h.Call("kv.set", map[string]any{"key": key, "value": receipt{}}, nil); err != nil {
			return pluginapi.Result{}, err
		}
		_ = h.Call("task.progress", pluginapi.Progress{Done: 1, Total: 1, Percent: 100, Message: label}, nil)
		return result, nil
	}
	if len(b) >= 44<<10 {
		cacheResult = pluginapi.Result{Message: label + "，完整结果见首次运行记录", Data: map[string]any{"action_id": action, "query_hash": hex.EncodeToString(sum[:12]), "response_omitted": true}}
	}
	if err := h.Call("kv.set", map[string]any{"key": key, "value": receipt{Until: now + ttl, Result: cacheResult}}, nil); err != nil {
		return pluginapi.Result{}, err
	}
	_ = h.Call("task.progress", pluginapi.Progress{Done: 1, Total: 1, Percent: 100, Message: label}, nil)
	return result, nil
}

// Normalize before computing the account-scoped cache key or calling the SDK.
func normalizeDianQuery(input map[string]any) error {
	for key, value := range input {
		if value == nil || value == "" {
			delete(input, key)
		}
	}
	if input["page"] == nil {
		input["page"] = 1
	}
	if input["size"] == nil {
		input["size"] = 50
	}
	if input["sort"] == nil {
		input["sort"] = "hot"
	}
	if input["media_type"] != "movie" && input["media_type"] != "tv" {
		return fmt.Errorf("请选择电影或剧集")
	}
	if input["media_type"] != "tv" {
		delete(input, "season")
	}
	for key, limits := range map[string][3]float64{
		"page": {1, 100000, 1}, "size": {1, 50, 1}, "tmdb_id": {1, 1e12, 1},
		"season": {0, 10000, 1}, "min_size_gb": {0, math.MaxFloat64, 0}, "max_size_gb": {0, math.MaxFloat64, 0},
	} {
		v, exists := input[key]
		if !exists {
			continue
		}
		b, _ := json.Marshal(v)
		var number float64
		if json.Unmarshal(b, &number) != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < limits[0] || number > limits[1] || limits[2] == 1 && number != math.Trunc(number) {
			return fmt.Errorf("请检查资源筛选参数：%s", key)
		}
		input[key] = number
	}
	if input["tmdb_id"] == nil && input["resource_id"] == nil {
		return fmt.Errorf("请选择作品或填写 TMDB ID")
	}
	if min, ok := input["min_size_gb"].(float64); ok {
		if max, ok := input["max_size_gb"].(float64); ok && min > max {
			return fmt.Errorf("最小体积不能大于最大体积")
		}
	}
	if value, exists := input["chn_sub"]; exists {
		switch value {
		case true, "true":
			input["chn_sub"] = true
		case false, "false":
			input["chn_sub"] = false
		default:
			return fmt.Errorf("请选择中文字幕条件")
		}
	}
	return nil
}
