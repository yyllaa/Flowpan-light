# 外部插件开发与接入

外部插件使用 Go/WASI，在 Flowpan 内调用共用宿主工具。安装包包含清单和 WASM；插件页面、配置、后台任务、授权及交互资源入口由项目提供。ABI 保持 v1，旧插件继续运行。

## 独立开发包

从本项目根目录导出一个新目录：

```powershell
python tools/plugin-kit.py --out ../my-plugin --id my-plugin --name 我的插件
```

导出目录含标准库依赖的 SDK、合同、示例源码、清单和构建脚本，可移出项目独立使用；不依赖本项目 go.mod 中的云盘或服务端依赖。

```powershell
cd ../my-plugin
go test ./sdk/pluginwasm
python build.py --host D:/实际位置/flowpan.exe --go go
```

Linux 的 `--host` 指向实际 flowpan 可执行文件。构建脚本使用 Go 的 WASI 目标，生成并校验 `plugin.zip`。本地调试阶段不要求仓库地址或 commit；在项目的「辅助插件 → 插件管理 → 已安装」导入并启用。导入后按实际用途设置账号、目录与工具授权。已有 ZIP 保留，发布新版本时改名或使用新输出目录。

## 通用执行入口

```go
func main() {
    pluginwasm.Serve(pluginwasm.Handlers{Run: run})
}

func run(call pluginwasm.Invocation) (pluginapi.Result, error) {
    if err := pluginwasm.ReportProgress(pluginapi.Progress{
        Done: 1, Total: 2, Message: "读取资源",
    }); err != nil { return pluginapi.Result{}, err }
    return pluginapi.Result{Message: "已完成"}, nil
}
```

`Serve` 统一处理 ABI、JSON 输入输出及异常，health/migrate 默认返回 ready，且不会调用 Run。插件需要数据迁移时显式填写 `Migrate`；有额外自检时填写 `Health`。处理函数返回 error 会转换为失败结果。原有 Read/Return/Call 用法继续支持。

运行记录显示每次动作结果，进度进入任务及插件日志。开发者只需报告关键步骤，避免逐文件打印大批日志。执行过程按原后台队列持久保存，插件无需自行维护重试按钮或轮询页面。

## 查询宿主能力

清单两层权限均声明 `host.read` 后，插件可查询当前执行的能力：

```go
host, err := pluginwasm.Capabilities()
if err != nil { return pluginapi.Result{}, err }
for _, tool := range host.Tools {
    if tool.Name == "browser.open" && !tool.Available {
        return pluginapi.Result{Message: "浏览器调用尚未就绪", Failed: true}, nil
    }
}
```

返回 API 版本、合同版本、请求响应上限及调用预算，并按本插件权限、提交时和当前工具开关、动作类型、确认状态和宿主实现给出 available/reason。能力可用表示可以尝试调用；具体账号、目录、凭据、浏览器安装状态仍由相应工具逐次核对。授权变化后重新查询，不持久缓存可用状态。此查询不执行工具，不增加云盘请求。

项目管理员也可运行 `flowpan plugin capabilities`，或在「插件管理 → 宿主能力」查看完整目录；管理目录本身不表示某个插件已获得授权。

## SDK 工具

SDK 为注册的宿主能力提供调用入口，结果类型仍可按业务使用结构体或 map 解码。

| 用途 | SDK |
| --- | --- |
| 能力 | Capabilities |
| 私有数据 | Get、Set、ListState |
| 进度与输出 | ReportProgress、TaskStatus、Log、Notify |
| HTTP 和服务 | Fetch、ServiceCall |
| 浏览器 | OpenBrowser、Browser.Evaluate、Browser.Close |
| 账号及只读文件 | Accounts、ListFiles、StatFile |
| 媒体 | MediaStatus、SearchMedia、ProbeMedia |
| 项目转存 | ParseTransfer、SubmitTransfer、TransferStatus |
| 项目整理 | RunOrganize、OrganizeStatus |

文件 ID 使用字符串。ListFiles 保留 offset/limit/cursor，以宿主返回的分页字段继续读取；不自行对每个文件重新列目录。浏览器、代理和 FFmpeg 复用项目维护入口，插件只声明需要的能力及站点。账号及目录沿用用户授权，提交项目转存和整理仍使用已确认的 run 动作和稳定业务幂等键。

## 网盘插件作为原生来源

如果插件要提供一个完整网盘，而不是只在插件窗口里搜索资源，使用 `definition.storage` 和 `pluginwasm.Handlers.Storage`。启用后宿主会把账号、文件浏览、STRM、整理、有声书、FUSE 和自动上传的来源选择器接到插件提供者；不需要修改每个页面，也不需要独立服务。网盘插件的字段、能力、账号生命周期、来源 ID、不透明游标和断点上传约束见 [外部网盘插件接入](网盘插件接入.md)。

快速导出模板：

```powershell
python tools/plugin-kit.py --out ../sample-drive --id sample-drive --name 示例网盘 --provider-id sampledrive --storage
```

该模板只带示例协议转发器，不含真实账号、默认插件源或供应商实现；替换 `plugins/examples/storage-http/main.go` 的协议后，先运行 SDK 测试，再用宿主的本地假服务做多账号和生命周期验证。

## 可识别错误

ABI 保留原有 `error` 文字，并附加 `error_code`。SDK 返回 `*pluginapi.HostError`，支持 errors.As，或通过 `pluginwasm.ErrorCode(err)` 判断。旧宿主只返回文字时使用 host_error。

| error_code | 含义及处理 |
| --- | --- |
| unknown_tool | 方法未注册，检查拼写及宿主版本 |
| permission_denied | 未声明权限，检查清单 |
| tool_disabled | 提交时或当前工具停用，调整授权后重新提交 |
| confirmation_required | 项目写入需要用户确认 |
| read_only | probe 动作调用了业务写入 |
| lifecycle | health/migrate 使用了业务工具 |
| unavailable | 当前宿主未挂接工具实现 |
| invalid_request / invalid_arguments | 调用 JSON 或参数格式错误 |
| call_limit / request_limit / response_limit | 调用预算、请求大小或响应大小超过上限 |
| cancelled / timeout | 调用被取消或已超时 |
| host_error | 工具返回的其他业务错误，保留原文字 |

这些错误不自动重放写入。转存和整理结果未知时，通过原幂等键查询回执；调整权限或配置后由用户决定重新提交。

## 本地打包及校验

```powershell
flowpan plugin pack --dir ./my-plugin --out ./my-plugin-1.0.0.zip
flowpan plugin check ./my-plugin-1.0.0.zip
```

pack 只收集 manifest.json 和清单指定的 WASM，生成 ZIP 前复用正式安装器校验清单、权限、路径、重复文件、体积与模块，并执行 WASM 编译检查。check 使用相同校验，不安装、不启用、不执行插件；实际导入仍执行原健康检查和更新回滚。命令输出 JSON，包含插件 ID、版本、SHA256 和 executed=false，适合接入构建流水线。

## 资源类插件与交互

在 definition.resources 指定搜索和解锁动作，搜索使用 probe，解锁使用需要确认的 run。搜索输入为 `media_type` 和 `tmdb_id`，结果的 data.resources 为 `[]pluginapi.Resource`；解锁输入为 resource_id，结果的 data.links/password 可交给项目转存入口。

启用的资源插件自动加入项目现有资源工作区和交互搜索，沿用片名候选、序号选择、来源隔离和转存整理交接；无需增加项目专属插件分支。各插件站点登录、签名和解锁实现仍放在插件内。完整参数及权限见 [宿主工具](plugin-host-tools.md)。


## 运行记录布局

内置插件、专用配置页和外部插件统一使用宿主的 `plugin-jobs` 组件。桌面记录按状态与任务名、时间与耗时、重试或取消、详情、删除固定列对齐；没有对应操作时保留槽位，不把其他操作挤到不同列。手机端将内容与时间操作分层，按钮保留可见文字。

声明式插件自动采用此布局，不需要在插件包内提供运行记录页面。宿主专用配置页接入时使用：

```html
<plugin-jobs v-if="tab==='records'"
  :plugin-id="definition.id" :request-json="requestJson" />
```

不要另写插件专用的记录列表或重放定时任务来填充页面；历史、计划来源和刷新由宿主队列提供。

## 定时任务注册

定时任务由宿主统一调度并写入持久化队列。插件不需要启动计时器，也不依赖第一个动作；同一个插件最多注册 16 条任务，任务可以绑定不同的动作、规则和开关。清单保存到安装数据库后生效，启用、停用、更新和卸载沿用插件生命周期。

在清单顶层和 `definition.permissions` 同时声明 `schedule`，在 `definition` 中加入：

```json
{
  "fields": [
    {"key": "daily_enabled", "label": "每日签到", "type": "boolean"},
    {"key": "daily_cron", "label": "签到时间", "type": "cron", "required": true,
     "visible_when": {"field": "daily_enabled", "equals": true}}
  ],
  "actions": [
    {"id": "status", "label": "检查连接", "kind": "probe"},
    {"id": "checkin", "label": "立即签到", "kind": "run", "requires_enabled": true}
  ],
  "schedules": [
    {"id": "daily", "label": "每日签到", "action": "checkin",
     "cron_field": "daily_cron", "enabled_field": "daily_enabled", "catch_up": true}
  ]
}
```

插件启用、所需配置完成、任务开关开启且 cron 有效时才执行。规则为六段 `秒 分 时 日 月 周`，秒固定为 0；例如 `0 0 8 * * *` 是每天 08:00，使用项目时区 Asia/Shanghai。支持列表、范围与步长，星期日可用 0 或 7；日期和星期同时限制时必须同时满足。保存配置前校验规则，错误规则不写入。任务 ID 在版本更新间应保持稳定。

`definition.schedules` 是注册入口；添加任务无需修改宿主调度器。开发包可用 `python tools/plugin-kit.py --out ../my-plugin --id my-plugin --schedule` 自动加入可选的每日任务字段和注册。首次安装保持任务关闭，用户配置 cron 后再开启。

宿主启动时检查当前分钟，此后对齐分钟检查，检查发生在第 20 或第 59 秒也会匹配该分钟。普通任务不补跑离线期间的历史分钟；签到类任务可设置 `catch_up: true`，宿主仅在启动时补当天最近一次已到期的任务，不补历史多天。同一次提交使用持久化回执，重复启动和清空运行记录不会重放；启动补执行前，当天使用同一配置手动成功完成的动作也计入回执，账号或配置变化后不复用。任务失败不会因下一次检查而在同一分钟自动重试；下一次 cron 到点仍正常运行。写操作因重启中断后沿用队列的结果待核验规则，不自动重放。

SDK 中可读取触发来源：

```go
func run(call pluginwasm.Invocation) (pluginapi.Result, error) {
    if trigger := call.Request.Schedule; trigger != nil {
        // ID identifies the registered schedule; ScheduledAt is RFC3339.
        pluginwasm.Log("定时任务：" + trigger.ID + "，计划时间：" + trigger.ScheduledAt)
    }
    return pluginapi.Result{Message: "执行完成"}, nil
}
```

手动执行时 `Request.Schedule` 为空。计划入队和执行前都会读取当前配置；关闭任务、停用插件、移除注册或更改 cron 后，未执行的旧任务不会继续调用业务动作。冻结的账号、输入和权限继续遵守现有队列合同。运行中动作沿用取消和超时机制。

`GET /api/plugins/v1/{id}` 返回的 `schedules` 提供任务 ID、动作、规则、是否启用、下次执行时间及规则错误；插件详情显示这些状态。运行记录返回 `schedule` 来源并保留任务日志。插件可继续自行调用 SDK Notify；若希望宿主在定时动作结束后发结果通知，可在注册中设置 `"notify": true` 并声明 `notification.send`，不要同时自行发送同一条结果通知。

兼容旧插件：没有显式 `schedules`，但存在 `scheduled` 布尔字段、`cron` 规则字段时，宿主仅选择明确的 `checkin` 动作，或唯一动作。多动作且没有 `checkin` 时不猜测执行动作。原有癫影安装无需重装；新包已显式注册签到动作。既有 115 签到、re0 和回收站规则继续读取原配置，无需重新登录或授权。

分享链接接管、原生转存协议和独立防护日志见 [插件分享接管与防护](插件分享接管与防护.md)。

## SDK 加密请求与私密配置

只需在 HTTP 请求中附加 Bearer Token 的插件继续使用 `auth_secret`，密钥由宿主注入。SDK 需要在插件内执行加密或签名时，额外声明 `secrets.read` 权限，并在对应的 `secret` 字段声明 `runtime_access: true`：

```json
{
  "permissions": ["http.fetch", "secrets.read"],
  "http_hosts": ["api.example.com"],
  "fields": [
    {"key": "api_key", "label": "API Key", "type": "secret", "runtime_access": true}
  ]
}
```

```go
key, err := pluginwasm.ReadSecret("api_key")
if err != nil { return pluginapi.Result{}, err }
response, err := pluginwasm.Fetch(pluginwasm.HTTPRequest{
    URL: "https://api.example.com/v2/query",
    Method: "POST",
    Headers: map[string]string{"Content-Type": "application/json", "X-API-Key": key},
    Body: encryptedPayload,
})
```

此能力只用于动作运行：health、migrate 和网盘 storage 入口均不开放；插件不能指定其他插件 ID，也不能读取未声明 `runtime_access` 的密码。宿主逐次检查启用状态、权限和工具授权；配置接口始终只返回密钥是否已设置。SDK 的明文密钥只在当前插件内存中使用，插件须避免写入结果、日志、KV 或安装包。

`http.fetch` 支持 `headers`，目标仍限定为 `http_hosts` 中的 HTTPS 站点，网络路径遵循下述声明。Host、代理认证、连接和消息长度等传输控制头不开放；跨站跳转不转发认证头，跨站 POST 跳转被拒绝。`probe` 动作仍只允许 GET；加密查询即使没有业务写入，也应声明为 `run` 并设置 `idempotent: true`，而不是放宽全局检测策略。

## 插件网络代理声明

插件在 `manifest.json` 的 `definition` 中声明网络路径，宿主统一提供连接与代理认证：

```json
"network": { "proxy": "direct" }
```

| proxy | 行为 |
| --- | --- |
| `direct` | 直连，忽略项目 HTTP 代理和系统代理 |
| `project` | 必须使用项目设置中的 HTTP 代理，未配置时提示先配置，不发送直连请求 |
| `inherit` | 兼容已有插件：配置了项目代理就使用，否则直连 |

未声明 `network` 的旧插件沿用 `inherit`；显式声明必须填写有效的 `proxy`，不接受代理 URL。网络策略在导入时校验，插件详情显示声明。新开发包默认声明直连，需要代理时使用 `python tools/plugin-kit.py --out ../my-plugin --id my-plugin --proxy project`。

此规则统一覆盖 `http.fetch`、`sdk.call`、浏览器页面及网盘插件由宿主执行的读写传输。代理地址、用户名和密码仅保存在项目设置，不下发给 WASM。更新项目代理后，新请求读取新配置；在途请求保持原连接。代理连接失败直接报告失败，不切换到直连。`service.call` 连接用户配置的辅助服务仍为直连，该服务自己的出站代理由服务管理。插件安装包和浏览器组件的下载继续使用项目下载配置，不受运行插件的网络声明影响。

## 私有协议由宿主执行

项目闭源而插件公开时，优先通过 `sdk.call` 对接宿主已登记的 SDK。仅在公开插件中保留业务代码、字段和 SDKRequest/SDKResponse；私有 SDK 不参与 WASM 链接。具体清单、参数表单和隔离规则见 [宿主 SDK 桥接](plugin-host-tools.md#宿主-sdk-桥接)。插件公开前同时检查源码包、WASM 和 ZIP，不只检查清单。

## 动作选项保存

动作的 `choices` 可用可选 `choice_field` 绑定同一插件已声明的 `select` 配置字段。字段选项值须与动作选项完全一致，字段必须可编辑且无条件隐藏；未保存时使用字段 `default` 或第一项。切换只修改配置草稿，点击保存配置后写入宿主；未绑定的旧动作保持临时选择。

手动运行仍提交实际选中的动作，目标动作的确认规则保持生效。定时注册保持稳定动作 ID；插件可在 `Request.Schedule` 非空时按冻结的 `Request.Config` 读取已保存方式，沿用定时任务确认及回执。

配置页将绑定动作放在该字段旁，顶部不重复渲染同一选项；其他标签保留快捷选项。两处共用草稿，切换标签不丢失未保存内容。手机端标签支持横向滑动，动作与选项保持同排和同高。

## 公开仓库构建

通用 SDK 位于 sdk/plugins。每个插件有自己的 go.mod，通过相对路径引用公共合同，不依赖主程序源码。测试时进入插件目录执行 go test ./...；在仓库根目录运行 python plugins/build.py --out dist/plugins 构建全部插件，也可以多次传 --plugin 只选择需要的目录。


## 后续插件通用接入

新版开发包随清单生成 release.notes 与 CHANGELOG.md，默认执行 pack --release 和 check --release。宿主从声明自动接入更新提示、仪表盘及 FUSE，SDK 通过 pluginapi.StorageError 保留网盘错误，不增加品牌分支。完整字段、兼容规则与验收方法见 [插件通用接入合同](plugin-integration-contract.md)。发布校验与新元数据需要支持该合同的宿主。
