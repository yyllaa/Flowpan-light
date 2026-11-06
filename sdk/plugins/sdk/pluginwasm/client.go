// Package pluginwasm implements the Flowpan command ABI. Build with GOOS=wasip1 GOARCH=wasm.
package pluginwasm

import (
	"encoding/json"
	"errors"
	"flowpan/app/pluginapi"
	"fmt"
	"io"
	"os"
)

type Invocation struct {
	ABIVersion string            `json:"abi_version"`
	Mode       string            `json:"mode"`
	Request    pluginapi.Request `json:"request"`
}

func Read() (Invocation, error) {
	var v Invocation
	e := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&v)
	if e == nil && v.ABIVersion != "1" {
		e = fmt.Errorf("unsupported ABI")
	}
	return v, e
}
func Return(result pluginapi.Result) {
	if e := json.NewEncoder(os.Stdout).Encode(result); e != nil {
		os.Exit(1)
	}
}
func Call(method string, params any, target any) error {
	return callHost(hostCall, method, params, target)
}

func callHost(transport func([]byte, []byte) uint32, method string, params any, target any) error {
	if params == nil {
		params = map[string]any{}
	}
	request, e := json.Marshal(map[string]any{"method": method, "params": params})
	if e != nil {
		return e
	}
	if len(request) == 0 || len(request) > pluginapi.MaxHostRequest {
		return fmt.Errorf("host request too large")
	}
	buffer := make([]byte, pluginapi.MaxHostResponse)
	n := transport(request, buffer)
	if n <= 0 || int(n) > len(buffer) {
		return fmt.Errorf("host response limit reached")
	}
	var response struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
		Code  string          `json:"error_code"`
	}
	if e = json.Unmarshal(buffer[:n], &response); e != nil {
		return e
	}
	if response.Error != "" {
		if response.Code == "" {
			response.Code = "host_error"
		}
		return &pluginapi.HostError{Code: response.Code, Method: method, Message: response.Error}
	}
	if target != nil && len(response.Data) > 0 {
		return json.Unmarshal(response.Data, target)
	}
	return nil
}

func ErrorCode(err error) string {
	var detail *pluginapi.HostError
	if errors.As(err, &detail) {
		return detail.Code
	}
	return ""
}
