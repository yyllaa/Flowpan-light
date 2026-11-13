package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"fmt"
	"strconv"
	"strings"
)

func resultText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "是"
		}
		return "否"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return string(v)
	case []any:
		values := []string{}
		for _, item := range v {
			if text := resultText(item); text != "" {
				values = append(values, text)
			}
		}
		return strings.Join(values, "、")
	}
	return ""
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := resultText(row[key]); text != "" {
			return text
		}
	}
	return ""
}

func responseRows(response map[string]any) []map[string]any {
	for _, key := range []string{"items", "files", "matches", "rows", "shares", "events", "entries", "records"} {
		if list, ok := response[key].([]any); ok {
			rows := []map[string]any{}
			for _, value := range list {
				if row, ok := value.(map[string]any); ok {
					rows = append(rows, row)
				}
			}
			return rows
		}
	}
	return nil
}

func sizeText(row map[string]any) string {
	for _, key := range []string{"total_size_bytes", "file_size", "size_bytes", "size"} {
		if value, ok := row[key].(float64); ok {
			if value < 1024 {
				return fmt.Sprintf("%.0f B", value)
			}
			if value < 1024*1024 {
				return fmt.Sprintf("%.1f KiB", value/1024)
			}
			if value < 1024*1024*1024 {
				return fmt.Sprintf("%.1f MiB", value/(1024*1024))
			}
			return fmt.Sprintf("%.2f GiB", value/(1024*1024*1024))
		}
	}
	return ""
}

// Only plain text from documented response fields reaches the host renderer.
func extensionResult(result *pluginapi.Result, action string, response, input map[string]any) {
	data := result.Data
	add := func(label, value string) {
		if value != "" {
			result.Items = append(result.Items, pluginapi.ResultItem{Label: label, Message: value})
		}
	}
	columns := []pluginapi.ResultField{}
	rows := []map[string]string{}
	switch action {
	case "profile":
		data["points"] = response["points"]
		member := "未提供"
		vip, hasVIP := response["vip_active"].(bool)
		if !hasVIP {
			vip, hasVIP = response["is_vip"].(bool)
		}
		if hasVIP {
			if vip {
				member = "有效"
			} else {
				member = "未开通"
			}
		}
		if response["vip_permanent"] == true {
			member = "永久会员"
		}
		data["membership"] = member
		add("昵称", firstText(response, "nickname", "username"))
		add("会员有效期", firstText(response, "vip_until"))
		if entitlement, ok := response["entitlements"].(map[string]any); ok {
			if membership := stringInput(entitlement["membership"]); membership != "" {
				labels := map[string]string{"normal": "普通会员", "free": "普通会员", "vip": "VIP", "permanent": "永久会员"}
				if text := labels[membership]; text != "" {
					data["membership"] = text
				} else {
					data["membership"] = membership
				}
			}
			for _, spec := range []struct {
				key    string
				fields [][2]string
			}{
				{"benefits", [][2]string{{"api_daily", "每日接口额度"}, {"api_minute", "每分钟接口额度"}, {"unlock_daily", "每日解锁额度"}}},
				{"remaining", [][2]string{{"api_daily", "剩余接口额度"}, {"unlock_daily", "剩余解锁额度"}}},
				{"growth", [][2]string{{"level", "成长等级"}, {"name", "成长称号"}, {"signin_days", "累计签到天数"}}},
			} {
				if values, ok := entitlement[spec.key].(map[string]any); ok {
					for _, pair := range spec.fields {
						add(pair[1], resultText(values[pair[0]]))
					}
					if spec.key == "remaining" {
						if v := resultText(values["api_daily"]); v != "" {
							data["quota"] = v
						}
					}
				}
			}
			add("额度重置时间", firstText(entitlement, "resets_at"))
			for _, pair := range [][2]string{{"growth_level", "成长等级"}, {"daily_api_limit", "每日接口额度"}, {"daily_api_used", "已用接口额度"}, {"daily_api_remaining", "剩余接口额度"}, {"daily_unlock_limit", "每日解锁额度"}, {"daily_unlock_used", "已用解锁额度"}} {
				add(pair[1], resultText(entitlement[pair[0]]))
			}
			if v := firstText(entitlement, "daily_api_remaining", "daily_api_limit"); v != "" {
				data["quota"] = v
			}
		}
	case "share-check":
		if exists, ok := response["exists"].(bool); ok {
			if exists {
				result.Message = "这条分享已发布"
			} else {
				result.Message = "尚未收录这条分享"
			}
		}
		add("分享码", stringInput(input["share_code"]))
		add("分享编号", firstText(response, "share_id", "id"))
	case "share":
		if id := firstText(response, "share_id", "id"); id != "" {
			data["share_id"] = id
		}
		if status := firstText(response, "status"); status != "" {
			data["share_status"] = status
		} else {
			data["share_status"] = "已发布"
		}
		add("作品", fmt.Sprintf("TMDB %v · %s", input["tmdb_id"], map[string]string{"movie": "电影", "tv": "剧集"}[stringInput(input["media_type"])]))
		add("资源标签", stringInput(input["tag"]))
	case "query", "rss":
		columns = []pluginapi.ResultField{{Key: "title", Label: "作品 / 资源"}, {Key: "type", Label: "类型"}, {Key: "tag", Label: "资源标签"}, {Key: "size", Label: "大小"}, {Key: "cost", Label: "积分"}, {Key: "id", Label: "分享编号"}}
		for _, row := range responseRows(response) {
			mediaType := firstText(row, "media_type")
			if mediaType == "movie" {
				mediaType = "电影"
			}
			if mediaType == "tv" {
				mediaType = "剧集"
			}
			rows = append(rows, map[string]string{"title": firstText(row, "title", "file_name", "name", "title_override"), "type": mediaType, "tag": firstText(row, "tag", "dt2_tag"), "size": sizeText(row), "cost": firstText(row, "unlock_cost", "cost"), "id": firstText(row, "share_id", "id")})
		}
		if action == "rss" {
			data["item_count"] = len(rows)
			data["next_cursor"] = response["next_cursor"]
			data["has_more"] = response["has_more"]
			data["more_label"] = "暂无"
			if response["has_more"] == true {
				data["more_label"] = "还有更新"
			}
		}
	case "virtual-cache", "sha1-query":
		columns = []pluginapi.ResultField{{Key: "name", Label: "文件"}, {Key: "size", Label: "大小"}, {Key: "sha1", Label: "SHA1"}, {Key: "status", Label: "状态"}, {Key: "path", Label: "目录 / 缓存键"}}
		for _, row := range responseRows(response) {
			rows = append(rows, map[string]string{"name": firstText(row, "file_name", "name", "file_id"), "size": sizeText(row), "sha1": firstText(row, "sha1", "file_sha1"), "status": firstText(row, "status", "exists", "found", "matched"), "path": firstText(row, "path", "file_path", "entry_key")})
		}
		for _, key := range []string{"matched_sha1s", "missing_sha1s", "exists_sha1s", "not_found_sha1s"} {
			if values, ok := response[key].([]any); ok {
				for _, v := range values {
					rows = append(rows, map[string]string{"sha1": resultText(v), "status": map[string]string{"matched_sha1s": "已收录", "exists_sha1s": "已收录", "missing_sha1s": "未收录", "not_found_sha1s": "未收录"}[key]})
				}
			}
		}
	case "rss-options":
		columns = []pluginapi.ResultField{{Key: "kind", Label: "筛选项"}, {Key: "value", Label: "值"}, {Key: "label", Label: "名称"}}
		var appendOptions func(string, any, int)
		appendOptions = func(kind string, value any, depth int) {
			if depth > 6 || len(rows) >= 500 {
				return
			}
			switch v := value.(type) {
			case []any:
				for _, item := range v {
					appendOptions(kind, item, depth+1)
				}
			case map[string]any:
				if name := firstText(v, "label", "name", "title"); name != "" {
					rows = append(rows, map[string]string{"kind": kind, "value": firstText(v, "value", "id", "key", "code"), "label": name})
					appendOptions(kind, v["children"], depth+1)
				} else {
					for key, item := range v {
						appendOptions(key, item, depth+1)
					}
				}
			default:
				if text := resultText(v); text != "" {
					rows = append(rows, map[string]string{"kind": kind, "value": text, "label": text})
				}
			}
		}
		for _, pair := range [][2]string{{"categories", "分类"}, {"regions", "地区"}, {"tags", "标签"}, {"presets", "预设"}, {"media_types", "作品类型"}} {
			appendOptions(pair[1], response[pair[0]], 0)
		}
	case "developer-usage":
		columns = []pluginapi.ResultField{{Key: "sdk", Label: "SDK"}, {Key: "operation", Label: "接口"}, {Key: "count", Label: "调用次数"}, {Key: "time", Label: "统计时间"}}
		for _, row := range responseRows(response) {
			rows = append(rows, map[string]string{"sdk": firstText(row, "sdk_name", "sdk_id", "name"), "operation": firstText(row, "operation", "endpoint", "path"), "count": firstText(row, "count", "requests", "total"), "time": firstText(row, "date", "day", "created_at", "time")})
		}
		add("总调用次数", firstText(response, "total_requests", "total_calls"))
	}
	if len(columns) > 0 {
		result.Table = &pluginapi.ResultTable{Columns: columns, Rows: rows}
	}
	if action == "virtual-cache" || action == "query" || action == "developer-usage" {
		if page, ok := input["page"].(float64); ok {
			size, _ := input["size"].(float64)
			more := size > 0 && float64(len(rows)) >= size
			if v, ok := response["has_more"].(bool); ok {
				more = v
			} else if total, ok := response["total"].(float64); ok {
				more = page*size < total
			}
			data["pagination"] = map[string]any{"page": page, "size": size, "has_more": more}
		}
	}
}
