package pluginapi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// InteractionDefinition registers entry points on the host's existing bots.
// Free text belongs to a plugin only after an explicit entry or session.set.
type InteractionDefinition struct {
	Commands         []InteractionCommand `json:"commands,omitempty"`
	MessageAction    string               `json:"message_action,omitempty"`
	AttachmentAction string               `json:"attachment_action,omitempty"`
}
type InteractionCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Action      string `json:"action"`
}
type InteractionAttachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type,omitempty"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
}
type InteractionCallback struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data,omitempty"`
}

// The host injects this context; action input cannot manufacture a conversation.
type InteractionInput struct {
	Delivery     string                  `json:"delivery"`
	Conversation string                  `json:"conversation"`
	Channel      string                  `json:"channel"`
	MessageID    string                  `json:"message_id"`
	Kind         string                  `json:"kind"`
	Text         string                  `json:"text,omitempty"`
	Command      string                  `json:"command,omitempty"`
	Arguments    string                  `json:"arguments,omitempty"`
	State        json.RawMessage         `json:"state,omitempty"`
	Callback     *InteractionCallback    `json:"callback,omitempty"`
	Attachments  []InteractionAttachment `json:"attachments,omitempty"`
}
type InteractionContext struct {
	InteractionInput
	NativeButtons      bool  `json:"native_buttons"`
	MaxAttachmentBytes int64 `json:"max_attachment_bytes"`
}
type InteractionButton struct {
	ID     string          `json:"id"`
	Label  string          `json:"label"`
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}
type InteractionReply struct {
	Delivery   string              `json:"delivery,omitempty"`
	Key        string              `json:"key"`
	Text       string              `json:"text"`
	Buttons    []InteractionButton `json:"buttons,omitempty"`
	TTLSeconds int                 `json:"ttl_seconds,omitempty"`
}
type InteractionSession struct {
	Delivery   string          `json:"delivery,omitempty"`
	Action     string          `json:"action"`
	State      json.RawMessage `json:"state,omitempty"`
	TTLSeconds int             `json:"ttl_seconds,omitempty"`
}
type InteractionSendFile struct {
	Delivery string    `json:"delivery,omitempty"`
	Key      string    `json:"key"`
	Input    HostInput `json:"input"`
	Caption  string    `json:"caption,omitempty"`
}

var interactionCommandName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func ReservedInteractionCommand(name string) bool {
	switch strings.ToLower(name) {
	case "status", "dashboard", "eventsync", "incremental", "strmincremental", "full", "fullsync", "strmfull", "strmfullsync", "plugin", "plugins", "reply", "cancel", "start", "help", "fp", "flowpan":
		return true
	}
	return false
}
func (d Definition) validateInteraction() error {
	if d.Interaction == nil {
		return nil
	}
	x := d.Interaction
	hasPermission := false
	hasRead := false
	for _, p := range d.Permissions {
		hasPermission = hasPermission || p == "interaction.receive"
		hasRead = hasRead || p == "interaction.read"
	}
	if d.Runtime != "wasm" || !hasPermission || !hasRead || len(x.Commands) > 32 || len(x.Commands) == 0 && x.MessageAction == "" && x.AttachmentAction == "" {
		return fmt.Errorf("插件交互声明无效或缺少接收权限")
	}
	validAction := func(id string) bool {
		for _, a := range d.Actions {
			if a.ID == id {
				return a.Kind == "run"
			}
		}
		return false
	}
	seen := map[string]bool{}
	for _, c := range x.Commands {
		if !interactionCommandName.MatchString(c.Name) || ReservedInteractionCommand(c.Name) || seen[c.Name] || !validAction(c.Action) || !utf8.ValidString(c.Description) || len([]rune(c.Description)) == 0 || len([]rune(c.Description)) > 100 || strings.ContainsAny(c.Description, "\r\n\x00") {
			return fmt.Errorf("插件交互命令无效：%s", c.Name)
		}
		seen[c.Name] = true
	}
	for _, id := range []string{x.MessageAction, x.AttachmentAction} {
		if id != "" && !validAction(id) {
			return fmt.Errorf("插件交互入口动作无效：%s", id)
		}
	}
	return nil
}

func interactionTools() []Tool {
	tools := []Tool{
		tool("interaction.context", "interaction.read", "交互", "读取宿主投递的当前会话、输入和附件元数据", "", false, false),
		tool("interaction.reply", "interaction.reply", "交互", "向当前会话回复文字和按钮；按键保存发送回执", "key:string text:string buttons?:array ttl_seconds?:number", true, false),
		tool("interaction.session.set", "interaction.session", "交互", "等待当前用户下一条输入并绑定已声明动作", "action:string state?:object ttl_seconds?:number", true, false),
		tool("interaction.session.end", "interaction.session", "交互", "结束当前插件的多轮输入和按钮等待", "", true, false),
		tool("interaction.attachment", "interaction.attachments", "交互", "将当前消息附件流式导入插件私有产物", "key:string id:string", true, false),
		tool("interaction.send_file", "interaction.reply", "交互", "将本插件产物、暂存或媒体结果发送到当前会话", "key:string input:object caption?:string", true, false),
	}
	tools[1].Input["properties"].(map[string]any)["buttons"].(map[string]any)["items"] = map[string]any{"type": "object", "required": []string{"id", "label", "action"}, "properties": map[string]any{"id": map[string]string{"type": "string"}, "label": map[string]string{"type": "string"}, "action": map[string]string{"type": "string"}, "data": map[string]string{"type": "object"}}}
	for n := range tools {
		tools[n].Input["properties"].(map[string]any)["delivery"] = map[string]string{"type": "string"}
	}
	tools[1].Input["properties"].(map[string]any)["buttons"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["data"] = map[string]any{}
	tools[2].Input["properties"].(map[string]any)["state"] = map[string]any{}
	return tools
}
