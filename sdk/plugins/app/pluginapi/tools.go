package pluginapi

import "strings"

// Tool is the single registry used by manifest validation, runtime authorization,
// the management screen and plugin development documentation.
type Tool struct {
	Name          string         `json:"name"`
	Permission    string         `json:"permission"`
	Group         string         `json:"group"`
	Description   string         `json:"description"`
	BusinessWrite bool           `json:"business_write"`
	Confirmation  bool           `json:"confirmation"`
	Input         map[string]any `json:"input_schema"`
}

func tool(name, permission, group, description, fields string, write, confirm bool) Tool {
	props := map[string]any{}
	required := []string{}
	for _, field := range strings.Fields(fields) {
		pair := strings.SplitN(field, ":", 2)
		key := pair[0]
		optional := strings.HasSuffix(key, "?")
		key = strings.TrimSuffix(key, "?")
		kind := pair[1]
		schema := map[string]any{"type": kind}
		if kind == "any" {
			delete(schema, "type")
		}
		if kind == "array" {
			schema["items"] = map[string]any{"type": "string"}
		}
		props[key] = schema
		if !optional {
			required = append(required, key)
		}
	}
	return Tool{name, permission, group, description, write, confirm, map[string]any{"type": "object", "properties": props, "required": required}}
}
func Tools() []Tool {
	return append([]Tool{
		tool("host.capabilities", "host.read", "宿主合同", "查询当前任务可用工具、限制及未就绪原因", "", false, false),
		tool("sdk.call", "sdk.call", "宿主 SDK", "调用声明的宿主 SDK；密钥、协议实现和网络连接由宿主管理", "provider:string operation:string input?:object", true, false),
		tool("secrets.read", "secrets.read", "私有数据", "读取本插件明确允许运行时访问的私密配置，用于 SDK 加密签名", "key:string", false, false),
		tool("kv.get", "data.read", "私有数据", "读取本插件私有值", "key:string", false, false),
		tool("kv.list", "data.read", "私有数据", "读取本插件私有数据", "", false, false),
		tool("kv.set", "data.write", "私有数据", "写入本插件缓存或检查点", "key:string value:any", false, false),
		tool("log", "log.write", "任务与通知", "记录插件日志", "message:string", false, false),
		tool("notify", "notification.send", "任务与通知", "通过项目通知渠道发送消息", "message:string", true, false),
		tool("task.progress", "task.progress", "任务与通知", "更新当前插件任务进度", "done?:number total?:number percent?:number message?:string checkpoint?:object", false, false),
		tool("task.status", "task.read", "任务与通知", "读取本插件的任务记录", "id:string", false, false),
		tool("http.fetch", "http.fetch", "网络", "通过项目代理请求声明的 HTTPS 站点", "url:string method?:string body?:string headers?:object auth_secret?:string", false, false),
		tool("service.call", "service.call", "网络", "调用插件显式配置的外部服务", "method:string path:string body?:object", false, false),
		tool("browser.open", "browser.use", "浏览器", "打开声明的站点，按插件和账号隔离登录状态", "url:string profile?:string", false, false),
		tool("browser.evaluate", "browser.use", "浏览器", "在当前任务页面运行脚本，支持宿主注入凭据", "session:string script:string args?:object secrets?:object", false, false),
		tool("browser.close", "browser.use", "浏览器", "释放当前任务的浏览器页面", "session?:string", false, false),
		tool("accounts.list", "accounts.read", "文件", "列出已授权的网盘及本机来源，不返回凭据", "", false, false),
		tool("files.list", "files.read", "文件", "分页读取已授权目录；有 root 时沿用主号索引接口", "account:string id?:string root?:string path?:string offset?:number limit?:number cursor?:string", false, false),
		tool("files.stat", "files.read", "文件", "读取已授权文件或目录详情", "account:string id:string", false, false),
		tool("files.capabilities", "files.read", "文件", "查询已授权账号的暂存、回写及冲突处理能力，不发起云盘请求", "account:string", false, false),
		tool("files.stage", "files.read", "文件", "将已授权云盘文件暂存到插件私有任务区，返回任务 ID", "key:string account:string id:string", true, false),
		tool("files.publish", "files.publish", "文件", "将本插件的媒体结果、通用产物或暂存文件回写；明确选择 skip、keep_both 或 replace", "key:string account:string parent_id:string name:string media_task?:string artifact?:string file_task?:string conflict:string original_task?:string", true, true),
		tool("files.task", "files.read", "文件", "查询本插件文件任务、回写阶段及备份引用", "id:string", false, false),
		tool("files.cancel", "files.read", "文件", "取消本插件文件任务；未知云盘写入结果保留待核验回执", "id:string", true, false),
		tool("files.release", "files.read", "文件", "释放本插件暂存文件，保留任务和原始快照回执", "id:string", true, false),
		tool("files.restore", "files.publish", "文件", "恢复本插件替换任务的原文件，并保留已处理文件", "id:string key:string", true, true),
		tool("media.status", "media.read", "媒体", "读取 FFmpeg、FFprobe 和浏览器就绪状态", "", false, false),
		tool("media.search", "media.read", "媒体", "复用项目 TMDB 设置查询电影或剧集", "query:string media_type:string", false, false),
		tool("media.probe", "media.read", "媒体", "探测已授权本机音频或视频，返回 JSON 元数据", "path?:string file_task?:string", false, false),
		tool("media.submit", "media.process", "媒体", "提交分段音视频转码、全部轨道 MKV 封装、标签或图片处理任务", "key:string path?:string file_task?:string operation:string format?:string at_seconds?:number width?:number bitrate_kbps?:number metadata?:object start_seconds?:number duration_seconds?:number audio_channels?:number sample_rate?:number map_all?:boolean", true, false),
		tool("media.task", "media.read", "媒体", "查询本插件媒体任务的进度和结果", "id:string", false, false),
		tool("media.result", "media.read", "媒体", "分段读取本插件已完成的媒体结果，每段最多 256 KiB", "id:string offset?:number limit?:number", false, false),
		tool("media.cancel", "media.process", "媒体", "取消本插件媒体任务并清理未完成输出", "id:string", true, false),
		tool("media.release", "media.process", "媒体", "释放本插件已结束的媒体输出；正在回写的结果保留到任务结束", "id:string", true, false),
		tool("transfer.parse", "transfer.read", "转存与整理", "本地识别 115、139、磁力等链接，不提交任务", "text:string", false, false),
		tool("transfer.submit", "transfer.write", "转存与整理", "按项目已保存的账号目标和整理设置提交转存任务", "account:string text:string key:string", true, true),
		tool("transfer.status", "transfer.read", "转存与整理", "查询本插件提交的转存任务与执行回执", "account:string key:string", false, false),
		tool("organize.run", "organize.write", "转存与整理", "按项目已保存的账号规则执行整理", "account:string key:string", true, true),
		tool("organize.status", "organize.read", "转存与整理", "读取授权账号的整理状态", "account:string", false, false),
	}, foundationTools()...)
}
func FindTool(name string) (Tool, bool) {
	for _, t := range Tools() {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
func KnownPermission(permission string) bool {
	if permission == "events.subscribe" || permission == "schedule" || permission == "storage.provider" {
		return true
	}
	for _, t := range Tools() {
		if t.Permission == permission {
			return true
		}
	}
	return false
}
func ToolDisabled(scope Scope, name string) bool {
	for _, v := range scope.DisabledTools {
		if v == name {
			return true
		}
	}
	return false
}
