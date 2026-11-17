package pluginapi

import "testing"

func TestShareRulesCanonicalizeOnlyDeclaredLinks(t *testing.T) {
	r := StorageShareRule{Host: "pan.wo.cn", PathPattern: `^/s/[A-Za-z0-9]+/`}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://pan.wo.cn/s/1H1R7H58757/", "https://pan.wo.cn/s/1H1R7H58757/4nwvEa5lAYx6:/点击文件链接或复制整段内容，打开「联通云盘APP」即可获取。"} {
		got, ok := r.Match(raw)
		if !ok || got != "https://pan.wo.cn/s/1H1R7H58757/" {
			t.Fatalf("canonical %q %v", got, ok)
		}
	}
	for _, raw := range []string{"http://pan.wo.cn/s/abc/", "https://pan.wo.cn.evil.test/s/abc/", "https://pan.wo.cn@evil.test/s/abc/", "https://pan.wo.cn:8443/s/abc/", "https://evil.test/?url=https://pan.wo.cn/s/abc/", "https://pan.wo.cn/not-a-share/"} {
		if _, ok := r.Match(raw); ok {
			t.Fatalf("matched %s", raw)
		}
	}
	q := StorageShareRule{Host: "share.future.test", PathPattern: `^/share/$`, QueryKeys: []string{"shareId", "shareCode"}}
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	got, ok := q.Match("https://share.future.test/share/?tracking=secret&shareId=abc&shareCode=XY12#ignored")
	if !ok || got != "https://share.future.test/share/?shareCode=XY12&shareId=abc" {
		t.Fatal(got, ok)
	}
}
func TestShareRulesCannotClaimBuiltinsOrBroadRoots(t *testing.T) {
	for _, r := range []StorageShareRule{{Host: "115.com", PathPattern: `^/s/.*`}, {Host: "yun.139.com", PathPattern: `^/s/.*`}, {Host: "*.evil.test", PathPattern: `^/s/.*`}, {Host: "127.0.0.1", PathPattern: `^/s/.*`}, {Host: "share.future.test", PathPattern: `/s/`}, {Host: "share.future.test", PathPattern: `^/.*`}} {
		if r.Validate() == nil {
			t.Fatalf("invalid rule accepted %+v", r)
		}
	}
}
