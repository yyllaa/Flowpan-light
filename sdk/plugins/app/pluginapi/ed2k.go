package pluginapi

// ED2KTask contains durable metadata only; credentials and download URLs never
// cross the plugin boundary. A complete record can be read by another plugin
// with access to its original account and directory.
type ED2KTask struct {
	ID         string `json:"id"`
	Account    string `json:"account"`
	ObjectID   string `json:"object_id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Done       int64  `json:"done"`
	Status     string `json:"status"`
	Hash       string `json:"hash,omitempty"`
	Link       string `json:"link,omitempty"`
	Message    string `json:"message,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Code       string `json:"code,omitempty"`
	RetryAt    int64  `json:"retry_at,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type ED2KQuery struct {
	Account string `json:"account,omitempty"`
	Query   string `json:"query,omitempty"`
	Status  string `json:"status,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type ED2KPage struct {
	Items  []ED2KTask `json:"items"`
	Total  int        `json:"total"`
	Offset int        `json:"offset"`
	Limit  int        `json:"limit"`
}

func ed2kTools() []Tool {
	return []Tool{
		tool("ed2k.submit", "ed2k.generate", "ED2K", "连续读取已授权文件生成 ED2K，复用已有结果并返回后台任务", "account:string id:string", true, false),
		tool("ed2k.list", "ed2k.read", "ED2K", "查询授权账号和目录内的 ED2K 结果库，不读取文件内容", "account?:string query?:string status?:string offset?:number limit?:number", false, false),
		tool("ed2k.task", "ed2k.read", "ED2K", "读取授权范围内的 ED2K 任务或已生成链接，其他插件可复用", "id:string", false, false),
		tool("ed2k.retry", "ed2k.generate", "ED2K", "明确重试已暂停任务，从完整分块检查点继续；冷却期间保持暂停", "id:string", true, false),
		tool("ed2k.cancel", "ed2k.manage", "ED2K", "停止授权范围内的 ED2K 读取，保留可继续的分块检查点", "id:string", true, false),
		tool("ed2k.remove", "ed2k.manage", "ED2K", "删除已结束的 ED2K 任务及链接记录，保留源文件", "id:string", true, true),
	}
}
