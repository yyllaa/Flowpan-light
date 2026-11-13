package wopan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

type QR struct {
	UUID  string `json:"uuid"`
	Image string `json:"image"` // data:image/png;base64,...
}

// QRStart 生成登录二维码
func (c *Client) QRStart(ctx context.Context) (QR, error) {
	data, err := c.restJSON(ctx, "GET", "/wohome/open/v1/QRCode/generate", nil, false)
	if err != nil {
		return QR{}, err
	}
	var r struct {
		UUID  string `json:"uuid"`
		Image string `json:"image"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return QR{}, fmt.Errorf("wopan: 二维码响应解析失败: %w", err)
	}
	if r.UUID == "" {
		return QR{}, errors.New("wopan: 未取得二维码 uuid")
	}
	if len(r.Image) > 0 && len(r.Image) > 22 && r.Image[:22] == "data:image/png;base64," {
		return QR{UUID: r.UUID, Image: r.Image}, nil
	}
	return QR{UUID: r.UUID, Image: "data:image/png;base64," + r.Image}, nil
}

// QRState 轮询扫码状态：1 待扫 / 2 已扫待确认 / 3 成功（返回 token）/ 4 过期
func (c *Client) QRState(ctx context.Context, uuid string) (int, string, string, error) {
	data, err := c.restJSON(ctx, "GET", "/wohome/open/v1/QRCode/query?uuid="+url.QueryEscape(uuid), nil, false)
	if err != nil {
		return 0, "", "", err
	}
	var r struct {
		State        any    `json:"state"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return 0, "", "", fmt.Errorf("wopan: 扫码状态解析失败: %w", err)
	}
	return int(toInt(r.State)), r.Token, r.RefreshToken, nil
}

// QRLogin 跑完整扫码流程；二维码通过 onQR 交给调用方展示（终端画/HTTP 返回都行）
func (c *Client) QRLogin(ctx context.Context, onQR func(QR), timeout time.Duration) error {
	qr, err := c.QRStart(ctx)
	if err != nil {
		return err
	}
	if onQR != nil {
		onQR(qr)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		state, tok, rt, err := c.QRState(ctx, qr.UUID)
		if err != nil {
			continue // 网络抖动不该中断扫码
		}
		switch state {
		case 3:
			if tok == "" {
				return errors.New("wopan: 扫码成功但没拿到 token")
			}
			return c.SetTokens(tok, rt)
		case 4:
			return errors.New("wopan: 二维码已过期，请重新获取")
		}
	}
	return errors.New("wopan: 扫码超时")
}
