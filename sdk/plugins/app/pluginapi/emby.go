package pluginapi

const EmbyMaxPageSize = 200
const EmbyMaxResponseBytes = 900 << 10
const EmbyMaxImageBytes = 16 << 20

// Emby calls bind a saved host connection. No URL or credential is accepted from
// the guest. IDs are local to InstanceID, including item, person and user IDs.
type EmbyInput struct {
	InstanceID      string            `json:"instance_id"`
	ID              string            `json:"id,omitempty"`
	UserID          string            `json:"user_id,omitempty"`
	Name            string            `json:"name,omitempty"`
	Query           map[string]string `json:"query,omitempty"`
	Key             string            `json:"key,omitempty"`
	Operation       string            `json:"operation,omitempty"`
	ExpectedVersion string            `json:"expected_version,omitempty"`
	Fields          map[string]any    `json:"fields,omitempty"`
	ImageType       string            `json:"image_type,omitempty"`
	Index           *int              `json:"index,omitempty"`
	Artifact        string            `json:"artifact,omitempty"`
	Updates         []EmbyMediaUpdate `json:"updates,omitempty"`
}
type EmbyMediaUpdate struct {
	Path       string `json:"Path"`
	UpdateType string `json:"UpdateType"`
}
type EmbyInstance struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
}
type EmbyItem struct {
	InstanceID string         `json:"instance_id"`
	Version    string         `json:"version"`
	Item       map[string]any `json:"item"`
}
type EmbyReceipt struct {
	InstanceID string `json:"instance_id"`
	Operation  string `json:"operation"`
	Key        string `json:"key"`
	State      string `json:"state"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (s Scope) HasEmbyInstance(id string) bool {
	for _, v := range s.EmbyInstances {
		if v == id && id != "" {
			return true
		}
	}
	return false
}
func embyTools() []Tool {
	const group = "Emby"
	tools := []Tool{
		tool("emby.instances.list", "emby.read", group, "列出已授权的 Emby 实例及连接就绪状态，不返回凭据", "", false, false),
		tool("emby.capabilities", "emby.read", group, "查询实例连接、服务版本与公开工具；远端操作按当前 Emby 权限执行", "instance_id:string", false, false),
		tool("emby.users.list", "emby.read", group, "读取实例用户 ID、名称及禁用状态", "instance_id:string", false, false),
		tool("emby.libraries.list", "emby.read", group, "读取媒体库或指定用户的媒体视图", "instance_id:string user_id?:string", false, false),
		tool("emby.items.list", "emby.read", group, "分页搜索媒体条目、媒体源与逐用户状态；query 使用 Emby 参数名", "instance_id:string user_id?:string query?:object", false, false),
		tool("emby.items.get", "emby.read", group, "读取完整条目及版本摘要，供选择性更新复核", "instance_id:string id:string user_id?:string", false, false),
		tool("emby.persons.list", "emby.read", group, "分页搜索人物及人物供应商标识", "instance_id:string query?:object", false, false),
		tool("emby.persons.get", "emby.read", group, "按名称读取人物资料及版本摘要", "instance_id:string name:string", false, false),
		tool("emby.persons.credits", "emby.read", group, "分页读取人物作品关联", "instance_id:string id:string query?:object", false, false),
		tool("emby.sessions.list", "emby.read", group, "读取完整会话列表，保留同一媒体的多个会话", "instance_id:string query?:object", false, false),
		tool("emby.images.list", "emby.read", group, "读取条目或人物的图片类型、索引与标签", "instance_id:string id:string", false, false),
		tool("emby.images.download", "emby.read", group, "将实例原图保存为本插件私有产物；需 artifacts.write/read 权限", "instance_id:string id:string image_type:string index?:number key:string", true, false),
		tool("emby.metadata.update", "emby.metadata.write", group, "复核版本后合并指定元数据字段，保存前值并回读验证；People 保留插件提供的角色关系", "instance_id:string id:string key:string expected_version:string fields:object", true, true),
		tool("emby.images.upload", "emby.images.write", group, "将本插件封存图片产物上传到指定条目或人物，保存操作回执", "instance_id:string id:string image_type:string index?:number artifact:string key:string", true, true),
		tool("emby.items.refresh", "emby.refresh", group, "提交单条目刷新并返回接受回执；不代表扫描完成", "instance_id:string id:string key:string query?:object", true, true),
		tool("emby.library.scan", "emby.refresh", group, "提交实例媒体库扫描并返回接受回执", "instance_id:string key:string", true, true),
		tool("emby.media.updated", "emby.refresh", group, "通知 Emby 其服务器内的媒体路径已变更", "instance_id:string key:string updates:array", true, true),
		tool("emby.tasks.list", "emby.read", group, "读取 Emby 计划任务状态、扫描进度和最后结果", "instance_id:string", false, false),
		tool("emby.operations.get", "emby.read", group, "读取本插件的 Emby 写入回执；待核验结果不自动重放", "instance_id:string operation:string key:string", false, false),
	}
	for i := range tools {
		if q, ok := tools[i].Input["properties"].(map[string]any)["query"].(map[string]any); ok {
			q["additionalProperties"] = map[string]any{"type": "string"}
		}
		if tools[i].Name == "emby.media.updated" {
			props := tools[i].Input["properties"].(map[string]any)
			props["updates"] = map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"Path", "UpdateType"}, "properties": map[string]any{"Path": map[string]any{"type": "string"}, "UpdateType": map[string]any{"type": "string", "enum": []string{"Created", "Modified", "Deleted"}}}}}
		}
	}
	return tools

}

func EventTypes() []string {
	return []string{"file.changed", "organize.completed", "audiobook.changed", "task.completed", "emby.library.changed", "emby.playback.changed"}
}
