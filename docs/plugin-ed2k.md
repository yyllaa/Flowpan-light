# ED2K 生成、结果库与插件调用

宿主版本 `2026.11.16-ed2k` 提供六个工具，ABI 仍为 v1。115 ED2K 是可单独安装、停用和维护的 WASI 插件，文件浏览入口、独立页面和发布插件的筛选组件均通过通用声明接入。

## 用户入口

通过插件市场安装 115 ED2K，或导入 `ed2k-1.0.3.zip`。在“已安装 → 授权范围”勾选所需的 115 账号。自动授权覆盖所选账号可访问的整盘，按当前插件独立保存。文件浏览勾选 1–50 个文件后，批量操作栏出现“生成 ED2K”；单文件菜单也有同一动作。目录或其他网盘不显示该插件入口。侧栏自动增加“ED2K 管理”，停用插件后入口移除。

管理页按名称、路径、哈希、账号和状态筛选，显示读取进度；支持停止、继续、单条复制、勾选复制和 TXT 导出。删除记录使用宿主确认弹窗，源文件保留。癫影 OpenAPI 自 1.3.9 起在分享发布工具的离线链接模式提供“已生成 ED2K”选项，可搜索、多选并与手填链接合并，发布仍经过原确认与供应商校验。

## 工具与权限

| 工具 | 权限 | 输入 | 结果 |
| --- | --- | --- | --- |
| `ed2k.submit` | `ed2k.generate` | `account`, `id` | ED2KTask，立即返回排队任务或已有结果 |
| `ed2k.list` | `ed2k.read` | 可选 `account`, `query`, `status`, `offset`, `limit` | ED2KPage，limit 默认 30、最多 100 |
| `ed2k.task` | `ed2k.read` | `id`（任务 ID） | ED2KTask |
| `ed2k.retry` | `ed2k.generate` | `id` | 从完整分块检查点继续 |
| `ed2k.cancel` | `ed2k.manage` | `id` | 停止读取并保留检查点 |
| `ed2k.remove` | `ed2k.manage`，需要确认 | `id` | 删除已结束任务及结果 |

状态为 `queued`, `running`, `cancelling`, `cancelled`, `paused`, `success`。结果包含账号 ID、对象 ID、文件名、路径、大小、已读字节、哈希和链接。`retry_at` 为冷却结束的 Unix 秒数。结果库保存在宿主插件数据目录的 `ed2k/library.db`，不保存媒体、凭据或直链。

生成需连续读取整个文件；生成 30 GiB 的文件会读取约 30 GiB 数据。宿主版本 `2026.11.16-ed2k-stable` 按稳定优先保留单连接，每个文件复用一块 256 KiB 读取缓冲，减少逐块申请与回收；不增加内容请求或下载并发。单 worker 串行处理，首次每文件一条连续内容流；无需按 64 KiB 反复请求。中断只持久化完整 9,728,000 字节块，继续时从该偏移请求剩余内容。重启后任务保持暂停，需明确继续。401、403、429 或供应商业务错误暂停当前账号的后续读取；403 冷却 300 秒，其余上述错误冷却 60 秒，不自动重试。1 TiB 单文件、200 个等待任务是本实现的工作上限，不是供应商风控阈值。

115 已验证算法使用 MD4 分块；整块大小的文件不追加空块。零字节、整块、两整块和跨块等九组向量均有独立预期值。已生成结果按算法、UID 绑定来源、对象 ID、大小和稳定内容标识复用；文件改名会更新名称和链接。宿主工具可服务符合读取合同的其他来源，115 专属插件自行限制其入口。

## 跨插件复用

其他插件声明 `ed2k.read` 即可读取其授权账号和目录内的共享结果，无需依赖 ED2K 插件进程或复制结果库。生成和继续还要求 `ed2k.generate`，停止和删除要求 `ed2k.manage`。每次调用与后台分块之间检查当前授权、安装包和冻结范围；账号更换 UID 后隐藏旧来源记录。查询列表不访问文件内容或重新获取云端文件信息，源文件已删除时已生成链接仍可查询。

```go
page, err := pluginwasm.ListED2K(pluginapi.ED2KQuery{
    Account: accountID, Query: "关键词", Status: "success", Limit: 20,
})
if err != nil { return pluginapi.Result{}, err }
for _, item := range page.Items {
    // 发布前再次 ED2KTask(item.ID)，以当前授权校验所选记录。
    fmt.Println(item.Name, item.Link)
}
```

不得用账号显示名称定位结果。离线链接可交给现有上传/分享工具；生成成功仅表示完整内容计算通过，不代表某个外部离线服务接受该文件。

## 文件浏览与动态选项合同

`definition.contributions` 新增 `point: "file-selection"`，原 `file-menu` 保持兼容。两者可用 `file_types: ["file"]` 和 `source_types: ["p115"]` 限定展示。宿主提供 `account`, `provider`, `source`, `ids` 及 `items`，混选目录时文件专属动作隐藏；服务端仍校验所有真实对象。`source_types` 接受通用提供者标识，不按插件 ID 分支。

动作设置 `placement: "context"` 后由文件浏览或独立页面调用，不占用插件主卡片的普通动作区。动作输入的 `multi-select` 可设置 `options_source` 为同插件另一个无危险声明的 `probe` 动作 ID，省略静态 options。宿主向该动作传 `query`, `offset`, `limit`，动作返回：

```json
{"data":{"options":[{"value":"record-id","label":"名称 · 路径"}],"total":1,"offset":0,"limit":20}}
```

通用组件负责搜索、分页、勾选、错误重试与加载状态。提交动作必须再次在服务端验证选项值；选项页面不是授权凭据。查询动作使用 `history_only: true`，不把定时读取结果当作待处理任务。

构建：`python plugins/build.py --plugin ed2k --go GO_EXE --host FLOWPAN_EXE --out dist/ed2k`。该脚本执行 WASI 编译及 `plugin pack/check --release`；插件附带 release.notes 与 CHANGELOG。外部开发包自动包含新增 SDK 文件及类型。
