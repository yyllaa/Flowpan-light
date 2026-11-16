// Package chineseconv 是 MusicTag v1.0.9.0 简繁转换的 Go 还原实现。
// 规格见 spec/chinese-conversion.md;行为经 oracle 对拍验证。
package chineseconv

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode"
)

// MappingChars 对应原始 MusicTagWinApp.ChineseUtils.MappingChars。
type mappingData struct {
	Chars    string   `json:"chars"`
	Lexemics []string `json:"lexemics"`
}

//go:embed data/tcmap.json
var tcmapJSON string

//go:embed data/tsmap.json
var tsmapJSON string

// Converter 是一个方向的转换器(简→繁 或 繁→简)。
type Converter struct {
	charTable []rune // 定长 20902,索引 = r - 0x4E00
	dict      map[string]string
	maxWord   int // 词表最大词长(rune 数)
}

// S2T 返回简→繁转换器(资源 tcmap)。
func S2T() (*Converter, error) { return newConverter(tcmapJSON) }

// T2S 返回繁→简转换器(资源 tsmap)。
func T2S() (*Converter, error) { return newConverter(tsmapJSON) }

func newConverter(embedded string) (*Converter, error) {
	var md mappingData
	if err := json.Unmarshal([]byte(embedded), &md); err != nil {
		return nil, err
	}
	c := &Converter{charTable: []rune(md.Chars), dict: map[string]string{}}
	for _, line := range md.Lexemics {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) >= 2 && parts[0] != "" {
			c.dict[parts[0]] = parts[1]
			if n := len([]rune(parts[0])); n > c.maxWord {
				c.maxWord = n
			}
		}
	}
	return c, nil
}

// char 换算:仅 一(U+4E00)..龥(U+9FA5) 走表,其余原样。
func (c *Converter) mapChar(r rune) rune {
	if r >= 0x4E00 && r <= 0x9FA5 && int(r-0x4E00) < len(c.charTable) {
		return c.charTable[r-0x4E00]
	}
	return r
}

// mapChars 逐字符走字表(保留空白等非 CJK 字符)。
func (c *Converter) mapChars(rs []rune, out *strings.Builder) {
	for _, r := range rs {
		out.WriteRune(c.mapChar(r))
	}
}

// isAllBlank 判断段是否全部为空白(unicode.IsSpace 覆盖半角/全角空格、tab、换行)。
func isAllBlank(rs []rune) bool {
	if len(rs) == 0 {
		return false
	}
	for _, r := range rs {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

type hit struct {
	start, end int
	repl       string
}

// Convert 完整模式转换(等价原始 DisablePredicate):最长匹配分词 + 分段组装。
func (c *Converter) Convert(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	// 1. 最长匹配分词
	var hits []hit
	for i := 0; i < len(runes); {
		matched := false
		upper := c.maxWord
		if i+upper > len(runes) {
			upper = len(runes) - i
		}
		for l := upper; l >= 2; l-- {
			if repl, ok := c.dict[string(runes[i:i+l])]; ok {
				hits = append(hits, hit{i, i + l, repl})
				i += l
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	if len(hits) == 0 {
		// 无命中:整串字级转换,空白保留
		var b strings.Builder
		b.Grow(len(s))
		c.mapChars(runes, &b)
		return b.String()
	}
	// 2. 分段组装:命中词之间的纯空白段丢弃,其余段字级转换
	var b strings.Builder
	b.Grow(len(s))
	prevEnd := 0
	for _, h := range hits {
		if seg := runes[prevEnd:h.start]; len(seg) > 0 && !isAllBlank(seg) {
			c.mapChars(seg, &b)
		}
		b.WriteString(h.repl)
		prevEnd = h.end
	}
	if tail := runes[prevEnd:]; len(tail) > 0 && !isAllBlank(tail) {
		c.mapChars(tail, &b)
	}
	return b.String()
}

// ConvertCharOnly 仅字级转换(等价原始 CheckPredicate),不使用词表。
func (c *Converter) ConvertCharOnly(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	c.mapChars([]rune(s), &b)
	return b.String()
}
