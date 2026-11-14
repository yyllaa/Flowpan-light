package pluginwasm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"flowpan/app/pluginapi"
)

type Handler func(Invocation) (pluginapi.Result, error)
type Handlers struct{ Run, Health, Migrate, Storage Handler }

// Serve owns command decoding and JSON output. Health/migrate never invoke Run.
func Serve(handlers Handlers) {
	if serve(os.Stdin, os.Stdout, handlers) != nil {
		os.Exit(1)
	}
}

func serve(input io.Reader, output io.Writer, handlers Handlers) error {
	result := func() (result pluginapi.Result) {
		defer func() {
			if recover() != nil {
				result = pluginapi.Result{Failed: true, Message: "插件执行异常"}
			}
		}()
		var invocation Invocation
		decoder := json.NewDecoder(io.LimitReader(input, pluginapi.MaxHostResponse+1))
		err := decoder.Decode(&invocation)
		if err == nil && decoder.Decode(&struct{}{}) != io.EOF {
			err = fmt.Errorf("插件调用输入格式错误")
		}
		if err == nil && invocation.ABIVersion != pluginapi.APIVersion {
			err = fmt.Errorf("unsupported ABI")
		}
		if err != nil {
			return pluginapi.Result{Failed: true, Message: err.Error()}
		}
		var handler Handler
		switch invocation.Mode {
		case "health":
			handler = handlers.Health
		case "migrate":
			handler = handlers.Migrate
		case "run":
			handler = handlers.Run
		case "storage":
			if invocation.Request.Storage == nil {
				return pluginapi.Result{Failed: true, Message: "缺少网盘调用上下文"}
			}
			handler = handlers.Storage
		default:
			return pluginapi.Result{Failed: true, Message: "插件调用模式无效"}
		}
		if handler == nil {
			if invocation.Mode == "run" || invocation.Mode == "storage" {
				return pluginapi.Result{Failed: true, Message: "插件缺少运行处理函数"}
			}
			return pluginapi.Result{Message: "ready"}
		}
		value, err := handler(invocation)
		if err != nil {
			value.Failed = true
			value.Message = err.Error()
			var failure *pluginapi.StorageError
			if invocation.Mode == "storage" && errors.As(err, &failure) && failure.Validate() == nil {
				if value.Data == nil {
					value.Data = map[string]any{}
				}
				value.Data["storage_error"] = failure
			}
		}
		return value
	}()
	return json.NewEncoder(output).Encode(result)
}
