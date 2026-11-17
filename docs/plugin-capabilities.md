# 插件能力提供与调用

插件使用 `definition.provides` 提供接口，使用 `definition.uses` 声明依赖。宿主负责发现、绑定、校验和执行，不解释接口名称，也不为插件品牌增加分支。接口可由插件独立实现，原有识别、整理、网盘、页面及交互入口继续按原合同运行。

## 提供一个接口

提供者清单在 `definition` 中声明普通动作与能力映射：

```json
{
  "actions": [{"id":"format","label":"格式化","kind":"probe","idempotent":true}],
  "provides": [{
    "id":"text-format",
    "interface":"example.text.format",
    "version":1,
    "action":"format",
    "input":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]},
    "output":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}
  }]
}
```

实现仍使用 SDK 的 `Handlers.Run`，读取 `Invocation.Request.Input`，返回 `Result.Data`。宿主在调用前校验输入，在成功完成时校验输出。提供者需要普通动作的全部权限与授权，不会因 `provides` 自动获得账号、目录、网络或密钥。

接口名使用带命名空间的稳定名称；`version` 是独立的正整数合同版本，与插件安装包版本无关。同一接口和版本的输入、输出及只读语义必须一致。破坏性修改发布新的接口版本；一个插件可以同时提供多个接口版本。只读接口映射到 `kind:"probe"`，读写接口映射到 `kind:"run"`。依赖已有结果才能运行的动作不可作为能力入口。

输入输出使用有界 JSON Schema 子集：`type`、`properties`、`required`、`items`、字符串 `enum`、`additional_properties`。字段名以 Go 合同的 JSON 标签为准，其中允许额外字段的键为 `additional_properties`。类型支持 object、array、string、number、integer、boolean、null；根必须为 object，数组必须声明 items。默认拒绝未声明字段。合同最大深度 6、节点 128；单次宿主输入受现有 64 KiB 限制，结果受现有响应大小限制。结构扩展应通过可选字段或新的接口版本完成。

## 依赖与自动绑定

调用方在 `definition` 中声明：

```json
{
  "permissions":["host.read","capabilities.call"],
  "uses":[
    {"id":"formatter","interface":"example.text.format","version":1},
    {"id":"optional-cover","interface":"example.cover.lookup","version":1,"optional":true}
  ]
}
```

包根同时声明 `"requires":{"features":{"plugins.capabilities":1}}`，旧宿主会在安装或启用前提示缺少合同。`optional` 缺项允许插件启用，调用方自行降级；必需接口缺项会阻止启用及新任务提交。

启用或提交任务时，宿主仅扫描本地安装声明，不发网盘及外部 API 请求。恰好一个已启用、版本匹配的提供者时自动绑定并保存；多个候选在「插件管理 → 授权范围 → 能力提供者」使用项目统一选择框绑定。绑定保存于既有授权数据 `scope.capability_bindings`，值为 `插件ID/提供接口ID`。

已经绑定的提供者停用、移除或不再提供该版本时，会显示缺项并阻止调用，保留原选择。新增插件不会自动切换已有绑定。用户清空选择后，下一次启用或提交时重新按唯一候选规则处理。可选接口也按相同规则绑定。当前提供者不处于运行状态时，清单上的声明不代表可以执行。

## 同步调用

```go
var out struct { Text string `json:"text"` }
if err := pluginwasm.CallCapability("formatter", map[string]any{"text":"hello"}, &out); err != nil {
    return pluginapi.Result{}, err
}
return pluginapi.Result{Data:map[string]any{"formatted":out.Text}}, nil
```

调用方只传自己声明的别名与输入，不能指定别的插件或任意动作。提供者使用自己的配置、密钥与私有数据；业务账号、目录和 Emby 实例权限取调用链各方提交时与当前授权的交集。撤销绑定、停用插件或升级参与方后，后续宿主操作及最终返回都会重新校验。执行中的任务不会自动扩大到新增授权。

同步调用继承父任务的超时、取消和进度上下文，共享 128 次宿主调用预算；最大委派深度 4，循环调用会被拦住。同一插件已有运行任务时返回 `capability_busy`，调用方稍后重试，嵌套调用不等待占用中的插件。检测与识别入口仅能调用只读能力；健康检查和迁移不执行插件间业务调用。危险动作需要父动作已经取得确认。

## 后台任务、恢复与幂等

长任务使用宿主已有持久任务队列，提供动作可以声明 `continuable:true` 并返回既有 `Continuation`。每步遵守原有运行时限；同步调用续执行动作会返回 `capability_async_required`。

```go
task, err := pluginwasm.SubmitCapability("formatter", "stable-business-key", input)
if err != nil { return pluginapi.Result{}, err }
// 在后续动作或续执行步骤中查询，不在一次 WASM 动作中持续轮询。
task, err = pluginwasm.GetCapabilityTask(task.ID)
// 用户要求取消时：pluginwasm.CancelCapabilityTask(task.ID)
```

相同调用方安装包、别名和幂等键复用同一任务，不重复提交；同一键改输入或提供者返回 `idempotency_conflict`。任务进入任务中心，进度、取消、续执行和重启恢复沿用原队列。任务绑定调用时的参与方包与接口，恢复前再次校验；更新后的调用方不接管旧版本提交的任务。查询与取消只接受当前调用方版本提交的任务。返回结构包含 id、status、provider、progress、data、message，剔除提供者私有 checkpoint。

主要错误码：`capability_unbound`、`capability_unavailable`、`capability_input`、`capability_output`、`capability_stale`、`capability_revoked`、`capability_busy`、`capability_cycle`、`capability_depth`、`capability_async_required`。未知别名、缺少权限、只读上下文、预算和确认继续使用既有宿主错误合同。使用 `pluginwasm.CapabilityStatuses()` 查询本插件依赖与状态。

## 外部引擎与新功能

音乐匹配、字幕算法、ED2K 管理、文件同步等业务接口可以使用自有命名空间，由插件提供和独立维护。调用项目现有工具仍通过原 SDK，如 files.stage、media.submit、files.publish、media.identify、organize 工具；不用复制已有流程。

需要 Python、GPU 或大型依赖的提供者可使用原有 `service.call` / `service.upload` 调用已配置外部服务，再以同样能力合同提供结果。`service.call` 是外部服务接口；`capabilities.call` 才是插件间调用。WASM 不直接安装系统软件或启动任意本机程序；依赖下载继续使用项目依赖工具，外部引擎继续使用服务配置。插件声明可以增加业务接口，新增宿主底层原语仍需要宿主实现。

提供者升级、回退、卸载和私有数据沿用原安装生命周期；能力绑定与授权一同备份恢复。插件内部算法和自定义业务数据使用 `extensions` 与私有存储更新，不新增宿主字段。不要用 `extensions` 冒充已实现的宿主工具。

## 生成独立开发包

```powershell
python tools/plugin-kit.py --out output/format-provider --id format-provider --capability-provider
python tools/plugin-kit.py --out output/format-consumer --id format-consumer --capability-consumer
```

两个目录均包含源 SDK、清单、构建脚本和本文，可在主仓库外单独构建发布。`--interface` / `--interface-version` 设置接口标识和合同版本。先安装并启用提供者，再启用调用方，即可验证自动发现与调用；开发包不需要带主项目源码。
