package wopan

import "testing"

// 用 Node crypto 独立算出的参考值，防止 sign/AES 实现漂移
func TestSignVector(t *testing.T) {
	got := signSum("AppQueryUser", "1700000000000", "123456", "api-user", "")
	const want = "7c3c5de3d0df00ac49619daa79ae5a29"
	if got != want {
		t.Fatalf("sign = %s, want %s", got, want)
	}
}

func TestAESVector(t *testing.T) {
	const key = "XFmi9GS2hzk98jGX"
	enc, err := aesEncrypt(`{"a":1}`, key)
	if err != nil {
		t.Fatal(err)
	}
	if enc != "fEhWhxcQMOQXzgHbQEA2jw==" {
		t.Fatalf("encrypt = %s", enc)
	}
	dec, err := aesDecrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != `{"a":1}` {
		t.Fatalf("decrypt = %s", dec)
	}
	// 抓包里的换行不能被当成密文
	if _, err := aesDecrypt("fEhWhxcQMOQXzgHb\r\nQEA2jw==", key); err != nil {
		t.Fatalf("换行容忍失败: %v", err)
	}
	// key 超过 16 字节要截断
	a, _ := aesEncrypt(`{"b":2}`, "0123456789abcdefZZZZ")
	b, _ := aesEncrypt(`{"b":2}`, "0123456789abcdef")
	if a != b {
		t.Fatal("长 key 未按前 16 字节截断")
	}
}

func TestParseHelpers(t *testing.T) {
	if toInt(float64(42)) != 42 || toInt("7") != 7 || toInt(nil) != 0 {
		t.Fatal("toInt")
	}
	if toString(float64(3)) != "3" || toString("x") != "x" {
		t.Fatal("toString")
	}
}
