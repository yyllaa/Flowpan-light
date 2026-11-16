package filenamerel

import "testing"

func TestValidateTemplate(t *testing.T) {
	cases := []struct{ tpl string; want string }{
		{"", "Msg_PatternCannotBeEmpty"},
		{"   ", "Msg_PatternCannotBeEmpty"},
		{"艺术家 - 标题", "Msg_ParamsInPatternNotFound"},
		{"@1 - @1", "Msg_ParamsInPatternCannotDuplicate"},
		{"@4@5 - @1", ""},                    // @4@5 合法
		{"@4@5", ""},
		{"@1@2", "Msg_ParamsInPatternCannotAdjacent"},
		{"@5@4 - @1", "Msg_ParamsInPatternCannotAdjacent"}, // 开头 @5@4
		{"@0", "Msg_ParamsInPatternNotFound"}, // @0 不计入参数([代码] 先查 @[1-8] 再查 @0)
		{"@0 - @1", "Msg_ParamsInPatternCannotUsePattern0"},
		{"@1 - @2", ""},
	}
	for _, c := range cases {
		if got := ValidateTemplate(c.tpl, false); got != c.want {
			t.Errorf("ValidateTemplate(%q) = %q, want %q", c.tpl, got, c.want)
		}
	}
	// @0 单独出现:不含 @[1-8] 参数 → NotFound(与参数检查顺序无关,[代码] :2941-2957)
	if got := ValidateTemplate("@0", true); got != "Msg_ParamsInPatternNotFound" {
		t.Errorf("ValidateTemplate(\"@0\", true) = %q", got)
	}
	if got := ValidateTemplate("@0 - @1", true); got != "" {
		t.Errorf("allowPattern0=true 时 @0+@1 应合法, got %q", got)
	}
}

func TestApplyTagToFilename(t *testing.T) {
	vals := map[string]string{
		"title": "标题", "artist": "A/B", "album": "专辑", "discstr": "1",
		"trackstr": "3", "year": "2024", "comment": "c", "albumartist": "AA",
	}
	get := func(f string) string { return vals[f] }
	cases := []struct{ tpl, want string }{
		{"@1", "标题"},
		{"@2 - @1", "A;B - 标题"},                   // 原代码 [\\/]→;(artist 内 "/" 也被替换)
		{"@5. @1", "03. 标题"},                       // @5 两位补零
		{"@1 (@3)", "标题 (专辑)"},
		{"@1/@2", "标题;A;B"},                        // 路径分隔符→分号
		{"@1: <2011>", "标题   2011 "},               // 非法字符→空格
		{"@6 - @1", "2024 - 标题"},
		{"@8/@2 - @1", "AA;A;B - 标题"},
		{"@1  multiple   spaces", "标题  multiple   spaces"}, // 空白逐字替换为空格(数量不变)
	}
	for _, c := range cases {
		if got := ApplyTagToFilename(c.tpl, get); got != c.want {
			t.Errorf("ApplyTagToFilename(%q) = %q, want %q", c.tpl, got, c.want)
		}
	}
}

func TestParseFilenameToTags(t *testing.T) {
	cur := map[string]string{
		"title": "", "artist": "", "album": "", "discstr": "",
		"trackstr": "", "year": "", "comment": "", "albumartist": "",
	}
	get := func(f string) string { return cur[f] }

	// 模板 "@2 - @1": "Artist - Title"
	ch := ParseFilenameToTags("@2 - @1", "周杰伦 - 晴天", get)
	if ch["artist"] != "周杰伦" || ch["title"] != "晴天" {
		t.Errorf("artist/title = %v", ch)
	}

	// @5 补零解析
	ch = ParseFilenameToTags("@5. @1", "03. 晴天", get)
	if ch["trackstr"] != "3" || ch["title"] != "晴天" {
		t.Errorf("track/title = %v", ch)
	}

	// @4@5 合并: 101 = disc 1, track 01
	ch = ParseFilenameToTags("@4@5. @1", "101. 晴天", get)
	if ch["discstr"] != "1" || ch["trackstr"] != "1" || ch["title"] != "晴天" {
		t.Errorf("disc/track/title = %v", ch)
	}

	// 值与现值相同 → 不写入
	cur2 := map[string]string{"title": "晴天"}
	get2 := func(f string) string { return cur2[f] }
	ch = ParseFilenameToTags("@1", "晴天", get2)
	if len(ch) != 0 {
		t.Errorf("相同值不应写入: %v", ch)
	}

	// @4@5 顺序非法已在 ValidateTemplate 拦截;此处验证正则构造
	rx, tokens := BuildTagRegex("@2 - @1")
	t.Logf("rx=%q tokens=%v", rx, tokens)
	if len(tokens) != 2 || tokens[0] != "@2" || tokens[1] != "@1" {
		t.Errorf("tokens = %v", tokens)
	}
}
