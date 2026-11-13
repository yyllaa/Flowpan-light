package main

import (
	"encoding/json"
	"errors"
	"flowpan/app/pluginapi"
	"fmt"
)

// This is the public host contract. No private SDK or credential is linked here.
type dianHostClient struct{ h host }

func (c dianHostClient) call(operation string, input map[string]any, out any) error {
	var reply pluginapi.SDKResponse
	if err := c.h.Call("sdk.call", pluginapi.SDKRequest{Provider: "dian115", Operation: operation, Input: input}, &reply); err != nil {
		return err
	}
	if reply.Failure != nil {
		return reply.Failure
	}
	if out == nil {
		return nil
	}
	if len(reply.Data) == 0 {
		return fmt.Errorf("OpenAPI 响应为空")
	}
	return json.Unmarshal(reply.Data, out)
}

type dianUser struct {
	ID           int            `json:"id"`
	Nickname     string         `json:"nickname"`
	Points       int            `json:"points"`
	AvatarURL    string         `json:"avatar_url"`
	Entitlements map[string]any `json:"entitlements"`
	Profile      map[string]any `json:"-"`
}

func (c dianHostClient) GetUserInfo() (*dianUser, error) {
	var raw json.RawMessage
	if err := c.call("me", nil, &raw); err != nil {
		return nil, err
	}
	var user dianUser
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &user.Profile); err != nil {
		return nil, err
	}
	return &user, nil
}
func (c dianHostClient) ListShares(input map[string]any, out any) error {
	return c.call("list-shares", input, out)
}
func (c dianHostClient) Signin(mode string, out any) error {
	return c.call("signin", map[string]any{"mode": mode}, out)
}
func (c dianHostClient) Unlock(input map[string]any, out any) error {
	return c.call("unlock", input, out)
}

func writeRejected(err error) bool {
	var failure *pluginapi.SDKFailure
	if errors.As(err, &failure) {
		return failure.Verified && failure.Status >= 400 && failure.Status < 500
	}
	var denied *pluginapi.HostError
	if errors.As(err, &denied) {
		switch denied.Code {
		case "invalid_arguments", "permission_denied", "sdk_credential", "confirmation_required", "plugin_revoked", "tool_disabled", "lifecycle", "read_only":
			return true
		}
	}
	return false
}
