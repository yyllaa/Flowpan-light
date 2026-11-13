package wopan

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// 与服务端一致的固定 IV（客户端 cryptoJS: iv="wNSOYIB1k1DjY5lA"）
const aesIV = "wNSOYIB1k1DjY5lA"

// sign = md5(key + resTime + reqSeq + channel + version)，version 恒为空串
func signSum(key, resTime, reqSeq, channel, version string) string {
	sum := md5.Sum([]byte(key + resTime + reqSeq + channel + version))
	return hex.EncodeToString(sum[:])
}

// getSuitableKey：超过 16 字节截断，否则原样（AES-128）
func suitableKey(k string) []byte {
	b := []byte(k)
	if len(b) > 16 {
		return b[:16]
	}
	return b
}

// AES-128-CBC + PKCS7，输出标准 base64（无换行）
func aesEncrypt(plain, key string) (string, error) {
	k := suitableKey(key)
	if len(k) != 16 {
		return "", fmt.Errorf("aes key 长度必须是 16 字节，当前 %d", len(k))
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	data := append([]byte(nil), []byte(plain)...)
	pad := aes.BlockSize - len(data)%aes.BlockSize
	data = append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, []byte(aesIV)).CryptBlocks(out, data)
	return base64.StdEncoding.EncodeToString(out), nil
}

// 解密：容忍抓包里的换行/空白和可能存在的引号包裹
func aesDecrypt(enc, key string) ([]byte, error) {
	s := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(strings.TrimSpace(enc))
	s = strings.Trim(s, `"`)
	k := suitableKey(key)
	if len(k) != 16 {
		return nil, fmt.Errorf("aes key 长度必须是 16 字节，当前 %d", len(k))
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		if raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err != nil {
			return nil, fmt.Errorf("base64 解码失败: %w", err)
		}
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return nil, errors.New("密文长度不是 16 的倍数")
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, []byte(aesIV)).CryptBlocks(out, raw)
	pad := int(out[len(out)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(out) {
		// 少数接口返回非 PKCS7 数据，原样返回便于排查而不是直接失败
		return out, nil
	}
	return out[:len(out)-pad], nil
}
