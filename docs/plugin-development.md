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

SDK 为全部 24 项宿主能力提供调用入口，结果类型仍可按业务使用结构体或 map 解码。

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

## 本仓库目录

独立 SDK 位于 `sdk/plugins/`，内含自己的 `go.mod`；在该目录执行 `go test ./sdk/pluginwasm`。根目录的 `tools/plugin-kit.py` 从该 SDK 导出新工程。癫影源码位于 `plugins/dian/`，使用相对模块引用复用 SDK，可在该目录执行 `go test ./...`。`python plugins/build.py --out dist/plugins` 从源码生成插件和 SDK 下载包。

[下载插件与 SDK](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06)。
