# 插件宿主工具 API v1

通用机器人交互新增 `interaction.context/reply/session.set/session.end/attachment/send_file` 六项工具；声明、权限、回执和 SDK 见 [插件交互合同](plugin-interaction.md)。媒体输入支持本插件私有 artifact，与 path/file_task 三选一。

所有外部 WASM 插件复用同一工具层。工具清单、权限、输入 JSON Schema 的唯一来源为 `app/pluginapi` 的工具目录。管理接口 `GET /api/plugins/v1/capabilities` 与“插件管理 → 宿主能力”读取同一清单；完整机器可读副本见 `plugin-host-tools.json`。

## 管理与授权

插件在 manifest 顶层和 `definition.permissions` 声明所需权限。“已安装 → 授权范围”设置账号和工具开关。账号取 `accounts.list` 的 ID（115 主号为 `main`、115 小号为账号 ID、139 为 `src_…`、本机为 `local`），不要用显示名称。目录范围与账号逐个绑定，沿用项目已保存的云端路径、目录 ID 和容器输出路径。不同账号不共用目录清单。界面不要求手填或选择目录，宿主从账号和项目原生业务配置自动确定范围；签到、搜索、扫码登录不要求目录配置。

每次调用检查声明权限、任务提交时授权和当前授权。新增授权不扩大已排队任务，撤销会收紧后续调用。工具开关可单独停用；未声明的工具不会因开关开启而获得权限。文件 ID 一律用字符串；宿主从真实父级链确认目录，插件传入的显示路径不作为授权依据。

通用宿主工具提供结构化的 `files.mkdir/rename/move/copy/delete`，逐次检查声明权限、来源实际能力、目录范围、确认和幂等回执。通过 `files.stage/publish` 搬运及回写字节，不接受任意宿主路径上传或返回云盘凭据。网盘驱动通过独立 storage 合同处理所属账号的供应商协议；该合同只发送对应账号的凭据。

插件可以调用项目已有的转存和整理任务。`transfer.submit` 使用所选账号在项目里保存的目标、去重及自动整理配置；`organize.run` 使用项目已保存的整理规则，不接受插件提供云盘操作或替换配置。两者需要相应权限、账号和目录授权、已确认的 run 动作及幂等键。目标与整理来源、输出均在调用前核对，probe、health、migrate 不执行这些任务。

站点签到和确认解锁由插件自己的站点动作处理。癫影解锁链接交给项目现有“转存下载”页面确认；其他插件也可声明权限后调用上述项目任务能力。probe 不发送业务通知，health/migrate 仅允许私有 KV 和日志。
## 自动目录范围

授权界面保留账号与工具开关，不设置额外的目录表单。保存时使用 `automatic_directories: true`，清除旧手工目录清单；旧客户端数据仍兼容。`account_directories` 和 `account_directory_ids` 作为执行任务内的账号与目录配对快照，不能由插件参数自行扩大。目录 ID 来自项目已保存的绑定，校验复用原有的真实父级链，不额外请求网盘来生成授权范围。

```json
{"accounts":["src_wopan_account","local"],"automatic_directories":true,"disabled_tools":[]}
```

网盘驱动只能申请宿主实际登记给该插件的提供者与来源；联通插件的授权列表和执行范围均不包含 115 或 139。旧记录即使含有其他网盘账号，执行时仍被过滤。普通资源插件按管理员选择的账号，继续跨网盘提交项目转存与整理。

云端范围取所属账号现有 STRM 根目录、转存目标、整理来源与输出、挂载根目录、自动上传目标；未绑定的目录不会因插件自报路径而开放。网盘驱动的本机访问仍限定在所属账号的 STRM 输出目录。普通资源插件选择本机来源后，还可复用原生本地任务、已配置下载器和有声书的现存本机目录，不依赖云盘账号；配置、日志和前端目录及覆盖这些目录的上级目录均被排除。本机删除将文件移入授权目录中的 `.flowpan-trash`，返回可定位的保留路径。

任务提交时冻结自动范围。每次调用重新计算当前绑定再取交集，账号删除、映射移除与工具撤销立即收紧；新增绑定不扩大已排队任务。非空 `account_directories` 仅按对应账号查询，空字典、缺键和空列表均拒绝文件访问。事件载荷中每个账号和目录均须通过校验。

WASM 不挂载宿主文件系统，本机读取额外经 `os.Root` 限制，符号链接或目录联接不能跳到其他输出。供应商原始 HTTP 协议仍由所属驱动持本账号凭据实现；宿主范围校验不等于解密并验证每个云端请求。上述隔离限定宿主工具与本机映射的边界。

## 工具

| 分类 | 工具 | 权限与行为 |
| --- | --- | --- |
| 宿主合同 | `host.capabilities` | `host.read`；返回当前执行的能力状态和资源限制 |
| 宿主 SDK | `sdk.call` | `sdk.call`；声明 SDK 和私密配置绑定，宿主执行协议并只返回业务结果 |
| 私有状态 | `kv.get`、`kv.list`、`kv.set` | `data.read` / `data.write`；按插件隔离，宿主回执保留键禁止覆盖 |
| 任务 | `task.progress`、`task.status` | `task.progress` / `task.read`；当前任务进度、本插件任务状态 |
| 输出 | `log`、`notify` | `log.write` / `notification.send`；项目日志与已有通知渠道 |
| 网络 | `http.fetch`、`service.call` | 项目代理、声明站点、宿主注入凭据；保留通用外部服务接入 |
| 浏览器 | `browser.open`、`browser.evaluate`、`browser.close` | `browser.use`；声明站点、插件/账号会话、任务租约 |
| 媒体读取 | `media.status`、`media.search`、`media.probe` | `media.read`；依赖状态、TMDB 搜索、已授权本机文件探测 |
| 媒体处理 | `media.submit`、`media.cancel` | `media.process`；结构化转码、抽帧和封面任务，不接收命令行 |
| 媒体结果 | `media.task`、`media.result` | `media.read`；查询本插件任务与分段读取完成结果，处理授权仍须有效 |
| 文件 | `accounts.list`、`files.list`、`files.stat` | `accounts.read` / `files.read`；115、139、本机已授权来源 |
| 媒体 | `media.status`、`media.search`、`media.probe` | `media.read`；工具就绪状态、已有 TMDB 查询、授权本机媒体 FFprobe |
| 转存 | `transfer.parse`、`transfer.submit`、`transfer.status` | `transfer.read` / `transfer.write`；解析链接、提交项目任务、查询本插件转存回执和实时任务状态 |
| 整理 | `organize.run`、`organize.status` | `organize.write` / `organize.read`；执行项目既有规则、查询账号整理状态 |

全部注册工具可由其他插件按声明调用。云盘列表分页复用项目缓存；`limit` 为 1–200。`files.list` 的 `id` 为目录 ID，本机则为绝对路径。旧插件的 `root` 主号索引合同仍兼容；新插件使用真实 ID。`files.stat` 返回真实路径。

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

网盘驱动的 `storage` 生命周期也可使用上述浏览器工具，仍须声明 `browser.use` 与 `browser.hosts`。宿主将 profile 固定绑定为当前网盘账号，再按插件隔离；驱动传入的 profile 不会切换到其他账号。扫码、验证和续期复用这个账号的浏览器会话，任务结束或取消释放页面。health、migrate 保持原有私有数据权限。

## FFmpeg 媒体任务

资源插件的 run 动作和网盘 storage 处理器均可调用 `media.status/probe/submit/task/result/cancel`。需要处理文件时同时声明 `media.read`、`media.process`，并由管理员授权账号或本机来源；目录遵循上述自动范围。扫码或浏览器登录不要求本机目录。FFmpeg 和浏览器继续按需准备，媒体任务不会隐式触发下载；声明 `dependencies.install` 后可显式调用 `dependencies.ensure`，复用项目的依赖管理。

```go
// 在 run 或 Storage 处理器中提交，inputPath 必须属于本次已授权本机范围。
task, err := pluginwasm.SubmitMedia(pluginapi.MediaProcess{
    Key: "cover-for-file-version-1",
    Path: inputPath,
    Operation: "cover", Format: "png", Width: 320,
})
if err != nil { return pluginapi.Result{}, err }
// 保存 ID 后，在下一次动作中查询，避免在一次 WASM 内持续轮询。
return pluginapi.Result{Data: map[string]any{"media_task": task}}, nil
```

`pluginwasm.MediaTask(id)` 返回状态、0–100 的进度、结果大小及时间；完成后用 `pluginwasm.MediaResult(id, offset, limit)` 分段读取 Base64 数据，默认和最大每段 256 KiB。需要终止时调用 `pluginwasm.CancelMedia(id)`。输出保存在本插件私有数据目录，插件无法指定输出位置，也没有任意 FFmpeg 参数、脚本、网络输入或宿主文件系统入口。

| operation | 参数 | 结果 |
| --- | --- | --- |
| `transcode` | `format` 为 mp3、m4a、flac、wav、mp4；可选 `bitrate_kbps` | 固定编解码参数的媒体文件 |
| `remux` | `format` 为 mp3、m4a、m4b、flac、wav、mp4、mkv、ogg、opus；可选结构化 `metadata` | 复制原有编码流、封装并写入标签，输入编码须与容器兼容 |
| `frame` | `format` 为 jpg/png，默认 jpg；可选 `at_seconds`、`width` | 指定时间的视频帧 |
| `cover` | `format` 为 jpg/png，默认 jpg；可选 `width` | 首个图像流的封面，支持音频内嵌封面和视频首帧 |

`remux` 的 `metadata` 支持 title、artist、album、album_artist、genre、date、track、disc、comment；每项最多 1024 字节且不含换行。封装任务不接受码率、尺寸或时间偏移，源文件保持不变。导航入口和独立前端通过同一插件动作调用这些能力，见 [插件独立页面](plugin-pages.md)。

任务键在插件内幂等，同一键与相同参数返回原任务；同键换参数会拒绝，不自动重试失败任务。新处理使用新键。全局最多 8 个未结束任务、同时处理 2 个、每个进程使用 2 个编解码线程；单任务最长 2 小时、结果上限 20 GiB，宽度上限 3840，码率上限 20000 kbps，时间偏移上限 7 天。完成状态为 `completed`，异常为 `failed`，主动取消、授权撤销或插件停用为 `cancelled`；重启前未结束的任务标记 `interrupted`，不自动重放。插件更新、回退和卸载会先停止并等待媒体进程退出；卸载保留数据时保留完成结果，勾选移除数据时一起清理。

宿主在打开输入时以目录句柄约束符号链接，并让 FFmpeg 读取已经授权的文件描述符；处理中撤销账号、工具或目录绑定会终止任务。处理权限失效后不能读取旧结果，更新后的插件也不能接管旧版本未结束任务。

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

声明 `events.subscribe` 后可订阅 `file.changed`、`organize.completed`、`audiobook.changed`、`task.completed`。载荷内全部账号与目录均通过配对授权后才投递，现有事件去重及任务检查点继续生效。站点业务放在外部插件，宿主只提供通用能力。

### 通用网盘文件事件

139、外部网盘及后续驱动通过宿主确认的文件操作进入统一 `file.changed` 持久发件箱。事件携带来源、账号授权版本、受影响父目录和对象 ID；旧的 115 事件载荷继续兼容。事件操作与订阅源功能无关。新载荷的 `data` 结构如下，`Event.ID` 用于消费去重：

```json
{
  "source": {"id":"src_example","provider":"example","account":"account-one","name":"示例账号","root_id":"root:opaque","revision":"authorization-revision"},
  "origin":"project",
  "change": {"id":"change:opaque","kind":"update","parent_ids":["parent:opaque"],"object_ids":["file:opaque"]}
}
```

`origin` 为 project 或 remote；kind 为 create、update、delete、move、reset。项目确认后的操作使用保守的 update，批量超限使用 reset；move 应包含原父目录和目标父目录。`Event.Cause` 在有任务上下文时记录来源任务 ID。插件只从 `Request.Event` 读取事件，不能发布任意来源事件或改写账号归属。

投递前重新核对插件启用、账号、当前来源 revision 和目录授权；子目录只沿宿主本地索引证明父级，不为投递调用云端遍历。来源变化的旧事件不会进入新账号，清理运行记录不清除投递去重依据。发件箱允许失败重放，插件消费应使用 Event.ID 或自己的幂等键，避免重复业务写入。

有真实供应商增量接口的驱动可额外声明 `storage.capabilities: ["list","stat","changes"]`，实现独立 changes 合同，见 [网盘增量事件](网盘插件接入.md#增量事件与游标)。未声明该能力的 139、联通等驱动不会为插件事件增加周期遍历；供应商客户端在项目外产生的变化，继续由现有原生扫描发现，不会虚构实时通知。

## 外部插件开发入口

通用 SDK 已封装全部工具，支持 Serve 生命周期入口、能力查询和结构化错误码。`flowpan plugin pack/check` 在本地打包和静态校验；独立开发包由 `tools/plugin-kit.py` 导出，不依赖主项目的服务端依赖。完整步骤见 [外部插件开发与接入](plugin-development.md)。

## 插件网络策略

插件通过 `definition.network.proxy` 声明 `direct`（直连）、`project`（必须使用项目 HTTP 代理）或 `inherit`（跟随项目设置）。未声明的旧插件保持 `inherit`。宿主对 HTTP、SDK、浏览器和网盘传输统一执行，插件不提供或读取代理地址。`project` 未配置代理时返回配置提示。辅助服务连接 `service.call` 保持直连。完整约定见 [插件网络代理声明](plugin-development.md#插件网络代理声明)。

## 宿主 SDK 桥接

私有协议实现留在宿主。插件声明 `sdk.call` 权限，以及 `definition.sdks: [{"provider":"dian115","credential":"openapi_key"}]`；credential 必须指向自己的 secret 字段，无需 `runtime_access` 或 `secrets.read`。

通用开发包使用 `pluginwasm.SDKCall(provider, operation, input)`，返回 SDKResponse 的业务 JSON、匿名账号指纹或结构化 SDKFailure。插件不能指定其他插件、任意 Key、站点或接口路径；宿主决定注册操作和固定出口，遵循插件声明的网络策略，沿用取消、大小限制、授权撤销和防护日志。失败保留站点的业务状态，响应验签后才标记 Verified。写入回执由业务插件按 Verified 和状态处理，结果不明时不自动重放。

`definition.actions[].placement="tools"` 将动作放入统一的扩展功能页；`input_fields` 声明 text、textarea、number、select、boolean、json、string-list、multi-select 参数。必填及格式错误显示在表单中，校验通过后发送 input；danger 动作冻结参数后确认再执行。后台仍独立校验允许的操作和参数。

## 资源筛选与分页

资源插件可在 `definition.resources.filter_action` 指定只读动作；动作必须为非 danger 的 probe，或幂等 run。未声明的插件沿用 `search_action`，无需修改。宿主使用该动作的 `input_fields` 渲染资源页筛选控件，不按插件 ID 判断。

`tmdb_id`、`media_type` 由已选择作品提供，`page`、`size` 由分页控件提供，其余字段按声明渲染。number 支持 `min`、`max`、`step`；`visible_when` 引用同一动作的字段，例如季数仅在 `media_type` 为 `tv` 时显示。隐藏字段不会提交，数字 0 和布尔 false 保留。后台仍负责业务参数校验。

筛选动作返回 `data.resources`，格式与普通搜索一致；分页返回 `data.pagination: {"page":1,"size":50,"has_more":true}`。每次只返回当前页，宿主不会自动遍历；翻页沿用已应用条件，修改或重置表单不自动请求。选择新作品或应用新条件从第一页开始，分页失败保留原页面。解锁和项目转存继续使用 `unlock_action` 及标准 links 结果。

### 扩展动作页面

无参数的顶部动作可用 `choices: [{value:"checkin",label:"普通签到"},{value:"signin-lucky",label:"幸运签到"}]` 声明执行方式。第一项必须指向当前动作，其余项指向本插件 `placement="tools"` 的独立动作；最多 8 项，不允许未知引用、重复或嵌套选项。宿主使用原生选择器，不因切换而发起请求；点击按钮后执行所选动作，沿用该动作的启用检查、确认要求和超时。未声明 choices 的插件保持原按钮，定时任务继续执行 schedules 中声明的动作。

外部插件默认声明界面，由宿主负责渲染。需要独立维护前端时，可选用 [插件独立页面](plugin-pages.md)，通过 definition.pages 声明侧栏入口和 ui/ 资源。动作的 `description` 展示功能说明；`input_groups` 使用 `{id,label,collapsible}`，字段通过 `group` 引用分组。可折叠分组有校验错误时会展开。

`string-list` 在页面使用每行一项的文本框，提交为去重后的字符串数组；`multi-select` 使用声明的 `options`，提交为字符串数组。后台插件仍须校验业务格式和数量，前端校验不能代替后端校验。

影视动作可声明 `media_input: {id_field:"tmdb_id",type_field:"media_type"}`，两者必须分别绑定已声明的 number 和 select 字段。宿主提供现有 TMDB 搜索，选择作品只填入参数；输入数字可直接选择 TMDB ID，仍须点击动作按钮执行。切换扩展页分别保留输入。

动作 `result_fields` 声明本动作的指标。`Result.table` 使用 `{columns:[{key,label}],rows:[{key:"文本"}]}`，宿主以纯文本表格展示并提供内部滚动；不执行单元格 HTML。`Result.items` 展示说明条目，原始业务响应默认折叠。`data.pagination` 为 `{page,size,has_more}`，已有 page 参数的动作可以手动翻页；RSS 的 `data.next_cursor` 与 `data.has_more` 提供“读取下一批”。仅本次已提交的条件未改变时允许继续，修改表单不会自动请求。危险动作确认页使用冻结的输入摘要。


## 云盘文件暂存与回写

资源插件可调用 `files.capabilities/stage/publish/task/cancel/release/restore`。文件通过宿主传输，以任务 ID 衔接 `media.probe`、`media.submit` 的 `file_task` 输入；`media.release` 释放已完成输出。回写需要 `files.publish` 权限和已确认的 run 动作，策略显式选择 skip、keep_both 或 replace；replace 保留原文件备份，当前为非原子交换。未知回执不重放。字段、能力与恢复方式见 [云盘文件处理合同](plugin-file-pipeline.md)。

## 通用插件运行基础

私有产物、宿主文件流、外部服务、AI、独立文件整理、下载器、依赖、CAS 和限时输出分享的完整合同见 [插件运行基础](plugin-runtime-foundation.md)。这些接口复用宿主实现，不修改已有整理、STRM、下载、识别的入口、配置和自动触发条件。`strm.create/inspect` 与 `organize.preview/apply` 处理插件选定的外部文件；已有 `strm.tasks/run/status`、`organize.run/status/stop` 继续服务项目原生任务。
