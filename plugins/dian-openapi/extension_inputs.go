package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var sha1Pattern = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
var codePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var coveragePattern = regexp.MustCompile(`^\d{1,5}(-\d{1,5})?(,\d{1,5}(-\d{1,5})?)*$`)

func inputLines(value any, label string, limit int) ([]string, error) {
	var values []string
	switch v := value.(type) {
	case nil:
	case string:
		if strings.HasPrefix(strings.TrimSpace(v), "[") {
			if json.Unmarshal([]byte(v), &values) != nil {
				return nil, fmt.Errorf("请检查%s", label)
			}
		} else {
			values = strings.Split(v, "\n")
		}
	case []string:
		values = v
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("请检查%s", label)
			}
			values = append(values, s)
		}
	default:
		return nil, fmt.Errorf("请检查%s", label)
	}
	result, seen := []string{}, map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	if len(result) > limit {
		return nil, fmt.Errorf("%s最多填写 %d 项", label, limit)
	}
	return result, nil
}

func integerInput(input map[string]any, key, label string, min, max float64, required bool) error {
	value, exists := input[key]
	if !exists {
		if required {
			return fmt.Errorf("请填写%s", label)
		}
		return nil
	}
	raw, _ := json.Marshal(value)
	var n float64
	if json.Unmarshal(raw, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < min || n > max {
		return fmt.Errorf("请检查%s", label)
	}
	input[key] = n
	return nil
}

func mediaInput(input map[string]any) error {
	if input["media_type"] != "movie" && input["media_type"] != "tv" {
		return fmt.Errorf("请选择电影或剧集")
	}
	return integerInput(input, "tmdb_id", "作品或 TMDB ID", 1, 1e12, true)
}

func shareCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") ||
			(u.Hostname() != "115.com" && u.Hostname() != "115cdn.com" && u.Hostname() != "anxia.com") {
			return "", fmt.Errorf("请输入有效的 115 分享链接")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != "s" {
			return "", fmt.Errorf("请输入有效的 115 分享链接")
		}
		value = parts[1]
	}
	if !codePattern.MatchString(value) {
		return "", fmt.Errorf("请输入有效的 115 分享码")
	}
	return value, nil
}

func normalizeExtension(action string, input map[string]any) error {
	switch action {
	case "share":
		return normalizeShare(input)
	case "virtual-cache", "sha1-query":
		if err := mediaInput(input); err != nil {
			return err
		}
		if action == "virtual-cache" {
			if input["page"] == nil {
				input["page"] = 1
			}
			if input["size"] == nil {
				input["size"] = 50
			}
			if err := integerInput(input, "page", "页码", 1, 100000, true); err != nil {
				return err
			}
			return integerInput(input, "size", "每页数量", 1, 200, true)
		}
		return normalizeSHA1s(input, true)
	case "share-check":
		code, err := shareCode(stringInput(input["share_code"]))
		if err != nil {
			return err
		}
		input["share_code"] = code
	case "rss":
		filter, err := objectInput(input["filter"])
		if err != nil {
			return err
		}
		for _, key := range []string{"media_types", "categories", "regions", "resolutions"} {
			if v, exists := input[key]; exists {
				values, err := inputLines(v, key, 100)
				if err != nil {
					return err
				}
				if key == "resolutions" {
					tags, err := objectInput(filter["tags"])
					if err != nil {
						return err
					}
					if len(values) > 0 {
						tags["resolution"] = values
					}
					if len(tags) > 0 {
						filter["tags"] = tags
					}
				} else if len(values) > 0 {
					filter[key] = values
				}
				delete(input, key)
			}
		}
		if types, exists := filter["media_types"]; exists {
			values, err := inputLines(types, "作品类型", 2)
			if err != nil {
				return err
			}
			for _, v := range values {
				if v != "movie" && v != "tv" {
					return fmt.Errorf("请选择电影或剧集")
				}
			}
			filter["media_types"] = values
		}
		input["filter"] = filter
		if input["limit"] == nil {
			input["limit"] = 50
		}
		return integerInput(input, "limit", "每批数量", 1, 50, true)
	case "developer-usage":
		if input["page"] == nil {
			input["page"] = 1
		}
		if input["size"] == nil {
			input["size"] = 100
		}
		for _, spec := range []struct {
			key      string
			min, max float64
		}{{"page", 1, 100000}, {"size", 1, 200}, {"sdk_id", 0, 1e12}} {
			if err := integerInput(input, spec.key, spec.key, spec.min, spec.max, false); err != nil {
				return err
			}
		}
		dates := map[string]time.Time{}
		for _, key := range []string{"since", "until"} {
			if value := stringInput(input[key]); value != "" {
				date, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return fmt.Errorf("时间请填写完整日期、时间和时区，例如 2026-11-01T00:00:00+08:00")
				}
				dates[key] = date
			}
		}
		if !dates["since"].IsZero() && !dates["until"].IsZero() && dates["since"].After(dates["until"]) {
			return fmt.Errorf("开始时间不能晚于结束时间")
		}
	}
	return nil
}

func stringInput(value any) string { s, _ := value.(string); return strings.TrimSpace(s) }

func normalizeSHA1s(input map[string]any, required bool) error {
	values, err := inputLines(input["sha1s"], "文件 SHA1", 500)
	if err != nil {
		return err
	}
	if required && len(values) == 0 {
		return fmt.Errorf("请填写文件 SHA1")
	}
	result, seen := []string{}, map[string]bool{}
	for _, value := range values {
		if !sha1Pattern.MatchString(value) {
			return fmt.Errorf("文件 SHA1 必须是 40 位十六进制字符")
		}
		value = strings.ToUpper(value)
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	if len(result) > 0 {
		input["sha1s"] = result
	} else {
		delete(input, "sha1s")
	}
	return nil
}

func normalizeShare(input map[string]any) error {
	if err := mediaInput(input); err != nil {
		return err
	}
	input["kind"] = "tmdb"
	if input["share_kind"] == nil {
		input["share_kind"] = "115"
	}
	switch input["share_kind"] {
	case "115":
		if link := stringInput(input["share_url"]); link != "" {
			if !strings.Contains(link, "://") {
				return fmt.Errorf("请输入完整的 115 分享链接")
			}
			if _, err := shareCode(link); err != nil {
				return err
			}
			input["share_url"] = link
		} else if code, err := shareCode(stringInput(input["share_code"])); err != nil {
			return err
		} else {
			input["share_code"] = code
		}
		if err := normalizeSHA1s(input, true); err != nil {
			return err
		}
		delete(input, "offline_url")
		delete(input, "offline_type")
	case "offline":
		links, err := inputLines(input["offline_url"], "离线链接", 500)
		if err != nil {
			return err
		}
		if len(links) == 0 {
			return fmt.Errorf("请填写磁力或 ED2K 链接")
		}
		kind := ""
		for _, link := range links {
			current := ""
			if u, err := url.Parse(link); err == nil && u.Scheme == "magnet" && u.Query().Get("xt") != "" {
				current = "magnet"
			}
			if strings.HasPrefix(link, "ed2k://|file|") && strings.HasSuffix(link, "|/") {
				current = "ed2k"
			}
			if current == "" {
				return fmt.Errorf("请填写有效的磁力或 ED2K 链接")
			}
			if kind != "" && kind != current {
				return fmt.Errorf("同批离线链接不能混用磁力和 ED2K")
			}
			kind = current
		}
		input["offline_url"] = strings.Join(links, "\n")
		input["offline_type"] = kind
		delete(input, "share_url")
		delete(input, "share_code")
		delete(input, "receive_code")
		if err := normalizeSHA1s(input, false); err != nil {
			return err
		}
	default:
		return fmt.Errorf("请选择分享方式")
	}
	// UI fields are converted into the documented DT2 tag, never sent as private parameters.
	if resolution := stringInput(input["resolution"]); resolution != "" {
		allowed := map[string]bool{"8K": true, "4K": true, "1080P": true, "1080I": true, "720P": true, "SD": true}
		if !allowed[resolution] {
			return fmt.Errorf("请选择清晰度")
		}
		tags := []string{"DT2", resolution}
		for _, key := range []string{"source_tag", "codec", "hdr", "audio"} {
			value := stringInput(input[key])
			if value != "" {
				if strings.ContainsAny(value, "|\r\n") {
					return fmt.Errorf("资源标签无效")
				}
				tags = append(tags, value)
			}
			delete(input, key)
		}
		sub := stringInput(input["subtitle_kind"])
		switch sub {
		case "raw":
		case "hardcoded_chs":
			tags = append(tags, "CHS", "HARDSUB")
		case "embedded_chs":
			tags = append(tags, "CHS", "MUXED-SUB")
		case "external_chs":
			tags = append(tags, "CHS", "EXT-SUB")
		default:
			return fmt.Errorf("请选择字幕类型")
		}
		if ext := strings.ToUpper(strings.TrimPrefix(stringInput(input["file_extension"]), ".")); ext != "" {
			input["file_extension"] = ext
			tags = append(tags, ext)
		} else {
			return fmt.Errorf("请选择文件格式")
		}
		input["tag"] = strings.Join(tags, "|")
	}
	delete(input, "resolution")
	tag := stringInput(input["tag"])
	if !regexp.MustCompile(`(?i)(^|\|)(8K|4K|1080P|1080I|720P|SD)(\||$)`).MatchString(tag) {
		return fmt.Errorf("资源标签中需要包含清晰度")
	}
	files, err := inputLines(input["file_list"], "文件列表", 500)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		input["file_list"] = files
	}
	if stringInput(input["file_extension"]) == "" && stringInput(input["file_name"]) == "" && len(files) == 0 && !regexp.MustCompile(`(?i)(^|\|)(MKV|MP4|ISO|TS|M2TS|AVI|MOV|FLV|RMVB|WMV|MPG|WEBM)(\||$)`).MatchString(tag) {
		return fmt.Errorf("请填写文件格式或文件名")
	}
	if input["media_type"] == "tv" {
		if input["season"] == nil {
			input["season"] = 1
		}
		if err := integerInput(input, "season", "季数", 0, 10000, true); err != nil {
			return err
		}
		if input["season"] == float64(0) && stringInput(input["seasons_csv"]) == "" {
			input["seasons_csv"] = "0"
		}
		for _, key := range []string{"seasons_csv", "episodes_csv"} {
			if v := stringInput(input[key]); v != "" && !coveragePattern.MatchString(v) {
				return fmt.Errorf("剧集范围请使用数字、逗号和范围，例如 1-12")
			}
		}
		if input["season"] == float64(0) && input["seasons_csv"] != "0" {
			return fmt.Errorf("特别篇的季范围应填写 0")
		}
	} else {
		for _, key := range []string{"season", "seasons_csv", "episodes_csv", "episode_count"} {
			delete(input, key)
		}
	}
	for _, key := range []string{"episode_count", "total_size_bytes", "unlock_cost_override"} {
		if err := integerInput(input, key, key, 0, 9007199254740991, false); err != nil {
			return err
		}
	}
	return nil
}
