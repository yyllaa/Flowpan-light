package wopan

import "testing"

// 扩名 → 客户端 fileType(0~5) 映射
func TestFileTypeOf(t *testing.T) {
	cases := map[string]string{
		"a.JPG":     "1",
		"b.mp4":     "2",
		"c.flac":    "3",
		"d.docx":    "4",
		"e.pdf":     "4",
		"f.xmind":   "5",
		"g.bin":     "0",
		"noext":     "0",
		"中文 视频.mkv": "2",
	}
	for name, want := range cases {
		if got := fileTypeOf(name); got != want {
			t.Fatalf("%s => %s, want %s", name, got, want)
		}
	}
}
