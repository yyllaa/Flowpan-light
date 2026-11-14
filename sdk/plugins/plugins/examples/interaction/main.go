// This scaffold exercises the host interaction contract without a business plugin.
package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() {
	pluginwasm.Serve(pluginwasm.Handlers{Run: func(i pluginwasm.Invocation) (pluginapi.Result, error) {
		c, err := pluginwasm.InteractionContext()
		if err != nil {
			return pluginapi.Result{}, err
		}
		switch i.Request.ActionID {
		case "run":
			if err = pluginwasm.WaitInteraction(pluginapi.InteractionSession{Action: "message"}); err != nil {
				return pluginapi.Result{}, err
			}
			err = pluginwasm.ReplyInteraction(pluginapi.InteractionReply{Key: "prompt", Text: "请输入内容或发送附件", Buttons: []pluginapi.InteractionButton{{ID: "finish", Label: "结束", Action: "done"}}})
		case "message":
			data := map[string]any{"text": c.Text}
			for _, input := range c.Attachments {
				artifact, e := pluginwasm.StageInteractionAttachment(input.ID, input.ID)
				if e != nil {
					return pluginapi.Result{}, e
				}
				data[input.ID] = artifact
			}
			if err = pluginwasm.EndInteraction(); err != nil {
				return pluginapi.Result{}, err
			}
			err = pluginwasm.ReplyInteraction(pluginapi.InteractionReply{Key: "received", Text: "输入已接收"})
			return pluginapi.Result{Message: "输入已接收", Data: data}, err
		case "done":
			if err = pluginwasm.EndInteraction(); err != nil {
				return pluginapi.Result{}, err
			}
			err = pluginwasm.ReplyInteraction(pluginapi.InteractionReply{Key: "ended", Text: "交互已结束"})
		}
		return pluginapi.Result{Message: "交互步骤完成"}, err
	}})
}
