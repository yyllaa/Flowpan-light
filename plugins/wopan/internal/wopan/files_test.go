package wopan

import "testing"

// 两种通道的文件字段不一样：dispatcher 给 size(数字)，pc 给 fileSize(字符串)
func TestParseFilesBothShapes(t *testing.T) {
	c := New(Config{})

	pc := []byte(`{"files":[
	 {"id":"1","fid":"F1","name":"a.mp3","fileSize":"11372307","type":1,"createTime":"20260313205110","fileType":"3","spaceType":"0"}
	],"hasNext":true}`)
	got, hasNext, err := c.parseFiles(pc)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNext || len(got) != 1 {
		t.Fatalf("pc: hasNext=%v n=%d", hasNext, len(got))
	}
	if got[0].Size != 11372307 || got[0].IsDir || got[0].Name != "a.mp3" {
		t.Fatalf("pc 解析错: %+v", got[0])
	}

	dispatcher := []byte(`{"files":[
	 {"id":"2","fid":"","name":"dirA","type":0,"size":0,"createTime":"20260101000000","fileVersion":7}
	]}`)
	got2, _, err := c.parseFiles(dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || !got2[0].IsDir || got2[0].Name != "dirA" || got2[0].FileVersion != "7" {
		t.Fatalf("dispatcher 解析错: %+v", got2)
	}
	if got2[0].ModTime != "20260101000000" {
		t.Fatalf("createTime 未回填: %+v", got2[0])
	}
}
