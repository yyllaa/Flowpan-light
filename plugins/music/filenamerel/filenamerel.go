// Package filenamerel 是 MusicTag v1.0.9.0 "文件名相关"模板引擎的 Go 还原实现。
// 规格见 spec/filename-engine.md;算法与反编译代码逐条对应。
package filenamerel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 参数字段映射
var paramFields = map[string]string{
	"@1": "title", "@2": "artist", "@3": "album", "@4": "discstr",
	"@5": "trackstr", "@6": "year", "@7": "comment", "@8": "albumartist",
}

// ValidateParam 反向写入校验:非空且 ≠ 现值才写入(返回要写入的值;不通过返回 ok=false)。
// 特殊:@4/@5(及 @4@5 合并)需 int 可解析。
func ValidateParam(param, value, current string) (field, final string, ok bool) {
	f, isKnown := paramFields[param]
	if !isKnown {
		return "", "", false
	}
	switch param {
	case "@4", "@5":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", "", false
		}
		return f, strconv.Itoa(n), true
	default:
		if strings.TrimSpace(value) == "" || value == current {
			return "", "", false
		}
		return f, value, true
	}
}

// ValidateDiscTrack 合并参数 @4@5:discstr = n/100, trackstr = n%100。
func ValidateDiscTrack(value string) (disc, track string, ok bool) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "", "", false
	}
	return strconv.Itoa(n / 100), strconv.Itoa(n % 100), true
}

// ValidateTemplate 模板校验(反向模式)。返回错误消息键;"" = 通过。
// 参见规格 §2;allowPattern0 对应"提取模式"。
func ValidateTemplate(tpl string, allowPattern0 bool) string {
	if strings.TrimSpace(tpl) == "" {
		return "Msg_PatternCannotBeEmpty"
	}
	seen := map[string]bool{}
	found := false
	for _, m := range regexp.MustCompile(`@[1-8]`).FindAllString(tpl, -1) {
		if seen[m] {
			return "Msg_ParamsInPatternCannotDuplicate"
		}
		seen[m] = true
		found = true
	}
	if !found {
		return "Msg_ParamsInPatternNotFound"
	}
	if !allowPattern0 && strings.Contains(tpl, "@0") {
		return "Msg_ParamsInPatternCannotUsePattern0"
	}
	// @4@5 先归一为 @4 再做相邻检查
	normalized := regexp.MustCompile(`^@4@5`).ReplaceAllString(tpl, "")
	normalized = strings.Replace(normalized, "@4@5", "@4", -1)
	if regexp.MustCompile(`@[0-8]@[0-8]`).MatchString(normalized) {
		return "Msg_ParamsInPatternCannotAdjacent"
	}
	if strings.HasPrefix(tpl, "@5@4") {
		return "Msg_ParamsInPatternCannotAdjacent"
	}
	return ""
}

// escapeTemplate 模板→正则转义(顺序与原始代码一致,见规格 §4.1)。
func escapeTemplate(tpl string) string {
	r := strings.NewReplacer(
		`\`, `\\`, `(`, `\(`, `)`, `\)`, `[`, `\[`, `]`, `\]`,
		`{`, `\{`, `}`, `\}`, `^`, `\^`, `$`, `\$`,
		`?`, `\?`, `*`, `\*`, `+`, `\+`, `.`, `\.`, `|`, `\|`,
	)
	return r.Replace(tpl)
}

var (
	tokenRe  = regexp.MustCompile(`(@[0-8])+`)
	splitRe  = regexp.MustCompile(`^(@4@5|@4|@5)(@[0-8])$`)
	numberRe = regexp.MustCompile(`^(\d*)(.*)$`)
)

// BuildTagRegex 模板模式:返回 (匹配文件名的正则, 参数 token 顺序)。
func BuildTagRegex(tpl string) (string, []string) {
	tokens := []string{}
	for _, m := range tokenRe.FindAllString(escapeTemplate(tpl), -1) {
		tokens = append(tokens, m)
	}
	rx := tokenRe.ReplaceAllString(escapeTemplate(tpl), "(.*)")
	return "^" + rx + "$", tokens
}

// ApplyTagToFilename 正向:标签→文件名(规格 §3)。
// values: 字段→值;track 在调用前应保证是数字。
func ApplyTagToFilename(tpl string, get func(field string) string) string {
	out := tpl
	repl := map[string]string{
		"@1": strings.TrimSpace(get("title")),
		"@2": get("artist"),
		"@3": strings.TrimSpace(get("album")),
		"@4": get("discstr"),
		"@5": fmt.Sprintf("%02d", atoi(get("trackstr"))),
		"@6": strings.TrimSpace(get("year")),
		"@7": strings.TrimSpace(get("comment")),
		"@8": strings.TrimSpace(get("albumartist")),
	}
	// 原始代码顺序:@1 @2 @3 @4 @5(@5 用 {0:D2})→ @6 @7 @8
	for _, k := range []string{"@1", "@2", "@3", "@4", "@5", "@6", "@7", "@8"} {
		out = strings.ReplaceAll(out, k, repl[k])
	}
	// 清理:路径分隔符→分号;空白与非法字符→空格
	out = regexp.MustCompile(`[\\/]`).ReplaceAllString(out, ";")
	out = regexp.MustCompile(`[\s":*?<>|]`).ReplaceAllString(out, " ")
	return out
}

// ParseFilenameToTags 反向:文件名→标签(模板模式,规格 §4.1)。
// 返回 字段→值 的变更集(仅含通过校验的写入)。
func ParseFilenameToTags(tpl, filename string, getCurrent func(field string) string) map[string]string {
	rxStr, tokens := BuildTagRegex(tpl)
	rx := regexp.MustCompile(rxStr)
	name := regexp.MustCompile(`\s`).ReplaceAllString(filename, " ")
	m := rx.FindStringSubmatch(name)
	if m == nil {
		return nil
	}
	changes := map[string]string{}
	for i, token := range tokens {
		if i+1 >= len(m) {
			break
		}
		value := strings.TrimSpace(m[i+1])
		if sub := splitRe.FindStringSubmatch(token); sub != nil {
			// token 形如 @4@N / @5@N:数字前缀与余下文本分开解析
			num, rest := "", value
			if nm := numberRe.FindStringSubmatch(value); nm != nil {
				num, rest = nm[1], nm[2]
			}
			if num != "" {
				if token == "@4@5" {
					// 合并解析:discstr = n/100, trackstr = n%100([代码] :1630-1636)
					if d, tr, ok := ValidateDiscTrack(num); ok {
						apply(changes, "discstr", d, getCurrent("discstr"))
						apply(changes, "trackstr", tr, getCurrent("trackstr"))
					}
				} else if f, v, ok := ValidateParam(sub[1], num, getCurrent(paramFields[sub[1]])); ok {
					apply(changes, f, v, getCurrent(f))
				}
			}
			if rest != "" {
				if f, v, ok := ValidateParam(sub[2], rest, getCurrent(paramFields[sub[2]])); ok {
					apply(changes, f, v, getCurrent(f))
				}
			}
			continue
		}
		if f, v, ok := ValidateParam(token, value, getCurrent(paramFields[token])); ok {
			apply(changes, f, v, getCurrent(f))
		}
	}
	return changes
}

func apply(changes map[string]string, field, value, current string) {
	if value != current {
		changes[field] = value
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
