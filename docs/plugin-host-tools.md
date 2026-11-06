# 插件宿主工具 API v1

所有外部 WASM 插件复用同一工具层。工具清单、权限、输入 JSON Schema 的唯一来源为 `app/pluginapi/tools.go`。管理接口 `GET /api/plugins/v1/capabilities` 与“插件管理 → 宿主能力”读取同一清单；完整机器可读副本见 `plugin-host-tools.json`。

## 管理与授权

插件在 manifest 顶层和 `definition.permissions` 声明所需权限。“已安装 → 授权范围”设置账号、目录和工具开关。账号取 `accounts.list` 的 ID（115 主号为 `main`、115 小号为账号 ID、139 为 `src_…`、本机为 `local`），不要用显示名称。目录范围应用于所选账号；本机填写容器实际绝对路径，云盘填写实际云盘路径。

每次调用检查声明权限、任务提交时授权和当前授权。新增授权不扩大已排队任务，撤销会收紧后续调用。工具开关可单独停用；未声明的工具不会因开关开启而获得权限。文件 ID 一律用字符串；宿主从真实父级链确认目录，插件传入的显示路径不作为授权依据。

插件不直接调用云盘的新建目录、改名、移动、复制、上传、删除接口，也不获取云盘凭据。旧插件声明原文件写入权限或传入 confirmed，也不能启用这些已移除的方法。

插件可以调用项目已有的转存和整理任务。`transfer.submit` 使用所选账号在项目里保存的目标、去重及自动整理配置；`organize.run` 使用项目已保存的整理规则，不接受插件提供云盘操作或替换配置。两者需要相应权限、账号和目录授权、已确认的 run 动作及幂等键。目标与整理来源、输出均在调用前核对，probe、health、migrate 不执行这些任务。

站点签到和确认解锁由插件自己的站点动作处理。癫影解锁链接交给项目现有“转存下载”页面确认；其他插件也可声明权限后调用上述项目任务能力。probe 不发送业务通知，health/migrate 仅允许私有 KV 和日志。
## 工具

| 分类 | 工具 | 权限与行为 |
| --- | --- | --- |
| 宿主合同 | `host.capabilities` | `host.read`；返回当前执行的能力状态和资源限制 |
| 私有状态 | `kv.get`、`kv.list`、`kv.set` | `data.read` / `data.write`；按插件隔离，宿主回执保留键禁止覆盖 |
| 任务 | `task.progress`、`task.status` | `task.progress` / `task.read`；当前任务进度、本插件任务状态 |
| 输出 | `log`、`notify` | `log.write` / `notification.send`；项目日志与已有通知渠道 |
| 网络 | `http.fetch`、`service.call` | 项目代理、声明站点、宿主注入凭据；保留通用外部服务接入 |
| 浏览器 | `browser.open`、`browser.evaluate`、`browser.close` | `browser.use`；声明站点、插件/账号会话、任务租约 |
| 文件 | `accounts.list`、`files.list`、`files.stat` | `accounts.read` / `files.read`；115、139、本机已授权来源 |
| 媒体 | `media.status`、`media.search`、`media.probe` | `media.read`；工具就绪状态、已有 TMDB 查询、授权本机媒体 FFprobe |
| 转存 | `transfer.parse`、`transfer.submit`、`transfer.status` | `transfer.read` / `transfer.write`；解析链接、提交项目任务、查询本插件转存回执和实时任务状态 |
| 整理 | `organize.run`、`organize.status` | `organize.write` / `organize.read`；执行项目既有规则、查询账号整理状态 |

全部 24 个工具可由其他插件调用。云盘列表分页复用项目缓存；`limit` 为 1–200。`files.list` 的 `id` 为目录 ID，本机则为绝对路径。旧插件的 `root` 主号索引合同仍兼容；新插件使用真实 ID。`files.stat` 返回真实路径。

`media.probe` 使用宿主已安装 FFprobe，固定参数、60 秒超时，只接受授权本机媒体；输出超过 768 KiB 时返回错误。`media.search` 不把 TMDB Key 交给插件。整理仅返回任务状态、数量和结果项目，不返回内部配置或密钥。

## 任务与数据

`task.progress` 报告当前插件任务进度，`task.status({id})` 只查询该插件自己的任务。私有 KV 缓存按插件隔离，不读取项目配置或其他插件的数据；`kv.set` 不覆盖宿主保留键。任务取消、版本检查、重试和事件去重沿用项目既有队列。

## 浏览器

在 `definition.browser` 中声明 `hosts` 和需要的 secret 字段。例如：

```json
{"permissions":["browser.use"],"browser":{"hosts":["example.com"],"credentials":["password"]}}
```

字段同时在 `definition.fields` 声明为 `type: secret`。hosts 为精确的主机和可选端口，不支持通配；生产站点使用 HTTPS。浏览器资源和重定向都受声明范围限制。插件的 profile 使用账号标识，宿主自动加插件隔离；不接受外部浏览器地址、任意执行文件或文件路径。

```go
page, err := pluginwasm.OpenBrowser("https://example.com/login", "account-one")
if err != nil { return err }
defer page.Close()
var result map[string]any
err = page.Evaluate(`async ({password}) => {
  // 站点脚本放在外部插件中；此处只返回业务状态。
  return {configured: password.length > 0}
}`, map[string]any{}, map[string]string{"password":"password"}, &result)
```

secrets 为“脚本参数名 → 插件声明的密钥字段”。宿主解密并注入，WASM 不读取明文；浏览器结果若含已声明凭据则拒绝返回。任务结束、取消或运行异常自动关闭页面；同一时刻一个浏览器任务，其余可取消地排队。浏览器空闲十分钟退出，登录会话保留。

浏览器维护位于“其他设置”，支持 CloakBrowser 官方源和 GitHub Releases，默认官方源；下载及 Linux 运行库使用项目代理。安装物位于 `/config/runtime/browser`，签名、SHA-256、启动检测通过后原子切换。主镜像不包含浏览器。可用 `FLOWPAN_BROWSER_BIN` 指向运维已提供的兼容浏览器。

## 插件调用与事件

SDK 为 `flowpan/sdk/pluginwasm`，已有 `Call(method, params, result)`；浏览器提供类型封装。其他工具按清单 JSON Schema 调用：

```go
var sources []map[string]any
err := pluginwasm.Call("accounts.list", map[string]any{}, &sources)
if err != nil { return err }
var parsed map[string]any
err = pluginwasm.Call("transfer.parse", map[string]any{"text": unlockedLink}, &parsed)
// 解锁链接返回插件资源工作区，由项目“转存 / 下载”按钮交给用户确认。
```

需要直接衔接项目任务的插件可声明 `transfer.write`、`transfer.read`、`organize.write`、`organize.read`，在带确认的 run 动作中调用：

```go
var result map[string]any
err := pluginwasm.SubmitTransfer("main", unlockedLink, "resource-123-transfer", &result)
if err != nil { return err }
err = pluginwasm.TransferStatus("main", "resource-123-transfer", &result)
// 单独执行已配置整理时使用 RunOrganize(account, key, &result)。
```

同插件、同方法、同幂等键复用持久回执；相同键更换账号、链接或项目配置会被拒绝。执行结果未知时保留待核验状态，不自动再次提交。`transfer.status` 仅查询该插件回执内的任务 ID；返回的任务状态来自原转存数据库，`completed` 回执表示调用已返回，云盘转存是否结束以 `tasks[].status` 为准。115 整理在当前插件后台任务中执行，139 沿用项目后台整理并通过 `organize.status` 查询进度。

单次宿主调用请求上限 64 KiB、响应 1 MiB，每个 WASM 执行最多 128 次宿主调用，内存 64 MiB。动作超时按定义限制在 60–600 秒。返回错误通过 ABI 的 `error` 字符串传递，SDK 转成 error；取消和任务版本变化沿用现有队列。

声明 `events.subscribe` 后可订阅 `file.changed`、`organize.completed`、`audiobook.changed`、`task.completed`。按授权账号、目录交集投递，现有事件去重及任务检查点继续生效。站点业务放在外部插件，宿主只提供通用能力。

## 外部插件开发入口

通用 SDK 已封装全部工具，支持 Serve 生命周期入口、能力查询和结构化错误码。`flowpan plugin pack/check` 在本地打包和静态校验；独立开发包由 `tools/plugin-kit.py` 导出，不依赖主项目的服务端依赖。完整步骤见 [外部插件开发与接入](plugin-development.md)。
