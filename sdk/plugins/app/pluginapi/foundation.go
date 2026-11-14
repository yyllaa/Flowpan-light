package pluginapi

// Artifacts are immutable, plugin-owned byte objects after sealing. They can be
// published as sidecars, manifests, subtitles or any other generated file.
type ArtifactCreate struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}
type Artifact struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Written int64  `json:"written"`
	SHA256  string `json:"sha256,omitempty"`
	Status  string `json:"status"`
}
type FileMutation struct {
	Key      string   `json:"key"`
	Account  string   `json:"account"`
	ID       string   `json:"id,omitempty"`
	IDs      []string `json:"ids,omitempty"`
	ParentID string   `json:"parent_id,omitempty"`
	Name     string   `json:"name,omitempty"`
}
type AIComplete struct {
	System          string `json:"system,omitempty"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
	JSON            bool   `json:"json,omitempty"`
}
type AICompletion struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// HostInput selects exactly one already authorized, immutable host output.
type HostInput struct {
	MediaTask string `json:"media_task,omitempty"`
	FileTask  string `json:"file_task,omitempty"`
	Artifact  string `json:"artifact,omitempty"`
}
type ServiceUpload struct {
	Path  string         `json:"path"`
	Input HostInput      `json:"input"`
	Field string         `json:"field,omitempty"`
	Body  map[string]any `json:"body,omitempty"`
}

type OrganizeIdentity struct {
	Title   string `json:"title,omitempty"`
	Year    string `json:"year,omitempty"`
	Type    string `json:"type,omitempty"`
	TMDBID  string `json:"tmdb_id,omitempty"`
	Season  int    `json:"season,omitempty"`
	Episode int    `json:"episode,omitempty"`
}
type OrganizePreview struct {
	Key      string                      `json:"key"`
	Account  string                      `json:"account"`
	IDs      []string                    `json:"ids"`
	OutputID string                      `json:"output_id"`
	AI       bool                        `json:"ai,omitempty"`
	TMDB     bool                        `json:"tmdb,omitempty"`
	Metadata map[string]OrganizeIdentity `json:"metadata,omitempty"`
}
type OrganizeNames struct {
	Directories []string `json:"directories"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
}
type OrganizePlanItem struct {
	ID           string        `json:"id"`
	OriginalName string        `json:"original_name"`
	Names        OrganizeNames `json:"names"`
	TargetID     string        `json:"target_id"`
	Conflict     bool          `json:"conflict"`
}
type OrganizePreviewResult struct {
	ID       string             `json:"id"`
	Account  string             `json:"account"`
	OutputID string             `json:"output_id"`
	Items    []OrganizePlanItem `json:"items"`
	Status   string             `json:"status"`
}

func foundationTools() []Tool {
	return []Tool{
		tool("strm.create", "artifacts.write", "STRM", "将插件提供的播放 URL 写成独立 STRM 产物，与项目 STRM 任务无关联", "key:string name:string url:string", true, false),
		tool("strm.inspect", "files.read", "STRM", "读取已授权的外部 STRM 文件并验证其中的播放地址", "account:string id:string", false, false),
		tool("organize.preview", "organize.read", "转存与整理", "为插件选择的外部 STRM、本机视频生成整理预览，复用项目识别和命名规则", "key:string account:string ids:array output_id:string ai?:boolean tmdb?:boolean metadata?:object", false, false),
		tool("organize.apply", "organize.write", "转存与整理", "执行本插件已预览的本机文件整理计划，保留 STRM 原始链接字节", "id:string key:string", true, true),
		tool("shares.create", "shares.write", "文件分享", "为本插件的宿主输出创建限时下载链接，支持 Range，停用或更新后失效", "key:string input:object expires_seconds?:number", true, true),
		tool("shares.revoke", "shares.write", "文件分享", "撤销本插件创建的下载链接", "id:string", true, false),
		tool("dependencies.status", "host.read", "运行依赖", "查询宿主管理的 FFmpeg、FFprobe 与浏览器安装状态", "", false, false),
		tool("dependencies.ensure", "dependencies.install", "运行依赖", "按项目已配置下载源准备 FFmpeg 或浏览器，复用正在执行的安装", "name:string", true, false),
		tool("downloads.list", "downloads.read", "下载器", "列出已配置的下载器身份，不返回连接地址或凭据", "", false, false),
		tool("downloads.status", "downloads.read", "下载器", "分页读取下载任务，复用项目下载器连接", "id:string filter?:string page?:number limit?:number", false, false),
		tool("downloads.add", "downloads.write", "下载器", "按下载器已保存的目录和分类提交下载，保存幂等回执", "id:string key:string urls:string paused?:boolean sequential?:boolean", true, true),
		tool("downloads.action", "downloads.write", "下载器", "暂停、继续、重新检查或重新通告下载任务", "id:string key:string action:string hashes:array", true, true),
		tool("service.upload", "service.call", "网络", "将宿主文件流式提交到已配置的外部服务，供识别、字幕及其他处理引擎使用", "path:string input:object field?:string body?:object", true, false),
		tool("artifacts.create", "artifacts.write", "私有产物", "预留本插件通用产物，支持字幕、清单和任意文件内容", "key:string name:string size:number sha256?:string", true, false),
		tool("artifacts.write", "artifacts.write", "私有产物", "按偏移写入产物；相同偏移和内容可重试，每段最多 32 KiB", "id:string offset:number base64:string", true, false),
		tool("artifacts.seal", "artifacts.write", "私有产物", "校验大小与 SHA256，将产物封存为不可变输入", "id:string", true, false),
		tool("artifacts.stat", "artifacts.read", "私有产物", "读取本插件产物状态及摘要", "id:string", false, false),
		tool("artifacts.read", "artifacts.read", "私有产物", "分段读取已封存的产物", "id:string offset?:number limit?:number", false, false),
		tool("artifacts.release", "artifacts.write", "私有产物", "释放本插件产物；回写正在使用时拒绝释放", "id:string", true, false),
		tool("files.read", "files.read", "文件", "按范围读取已授权云盘或本机文件，每段最多 256 KiB", "account:string id:string offset?:number limit?:number", false, false),
		tool("files.mkdir", "files.write", "文件", "在已授权目录创建子目录，保存幂等回执", "key:string account:string parent_id:string name:string", true, true),
		tool("files.rename", "files.write", "文件", "重命名已授权文件或目录，保存幂等回执", "key:string account:string id:string name:string", true, true),
		tool("files.move", "files.write", "文件", "在同一来源的授权目录间移动对象", "key:string account:string ids:array parent_id:string", true, true),
		tool("files.copy", "files.write", "文件", "复用来源原生复制；跨来源使用暂存和回写", "key:string account:string ids:array parent_id:string", true, true),
		tool("files.delete", "files.delete", "文件", "按来源原生删除语义删除已授权对象，保存回执且不重放未知写入", "key:string account:string ids:array", true, true),
		tool("media.identify", "media.read", "识别与 AI", "复用项目文件名解析、季集和媒体命名识别；可选择已有 AI 与 TMDB", "name:string ai?:boolean tmdb?:boolean", false, false),
		tool("ai.status", "ai.use", "识别与 AI", "查询已配置 AI 的模型和协议，不返回地址或凭据", "", false, false),
		tool("ai.complete", "ai.use", "识别与 AI", "通过项目 AI 配置、代理和限流执行通用文本生成", "system?:string input:string max_output_tokens?:number json?:boolean", true, false),
		tool("cas.inspect", "files.read", "CAS", "解析并验证项目兼容的 CAS 描述内容", "text:string", false, false),
		tool("cas.create", "artifacts.write", "CAS", "根据已验证的文件暂存摘要生成可回写的 CAS 产物", "key:string file_task:string name?:string", true, false),
		tool("strm.tasks", "strm.read", "STRM", "列出云端和本机范围均已授权的现有 STRM 任务", "", false, false),
		tool("strm.run", "strm.write", "STRM", "按现有 STRM 配置提交宿主后台任务", "id:string key:string mode?:string", true, true),
		tool("strm.status", "strm.read", "STRM", "读取授权 STRM 任务的运行记录", "id:string", false, false),
		tool("organize.stop", "organize.write", "转存与整理", "请求停止已授权来源的项目整理任务", "account:string key:string", true, true),
	}
}
