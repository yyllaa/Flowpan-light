# Emby 插件调用合同

宿主按 `instance_id` 复用「Emby」页面保存的连接、API Key 和项目 HTTP 代理。SDK 不接收实例地址、API Key、用户密码或通知密钥。媒体库目录继续由 Emby 管理；本合同使用 Emby 内的路径，不增加目录映射，不改变播放、302、STRM、用户播放规则与大小号路由。

## 授权与发现

资源插件在 `definition.permissions` 及顶层 permissions 声明需要的权限：

- `emby.read`：实例、连接能力、用户、媒体库、条目、人物、会话、图片信息、任务及本插件写入回执。
- `emby.metadata.write`：元数据更新，包括 People、人物自己的 ProviderIds、锁定字段。
- `emby.images.write`：图片上传。
- `emby.refresh`：文件变更通知、单条目刷新、媒体库扫描。
- 原图下载另需 `artifacts.write`；图片读取、上传及发布另需 `artifacts.read`。产物按插件和安装包版本隔离。
- 订阅另需 `events.subscribe`，读取事件中的实例需同样取得实例授权。

在「插件管理 → 已安装 → 授权」中选择 Emby 实例。授权是**整个所选实例**，包括该实例的用户和媒体库；当前不提供用户级或单媒体库级子范围。没有默认实例授权，不使用空 ID 回落。网盘驱动不使用这套实例授权。

提交时冻结实例列表，每次调用与当前授权取交集。停用、替换安装包、取消任务、撤销工具或实例授权后，后续出站请求停止；连接地址或密钥在一次调用中发生变化时返回 `instance_changed`。宿主实际出站请求进入统一插件仪表盘统计；实例发现、授权拒绝及回执重放不增加远端请求。

`host.capabilities` 返回工具可用原因，例如 `permission_denied`、`instance_required`、`not_ready`、`confirmation_required`。`emby.instances.list` 只列出已授权实例 ID、名称、ready/reason；`emby.capabilities` 会请求所选服务器版本。列出的工具是宿主支持的合同，目标 Emby 的 API Key 权限、版本和插件仍决定远端是否接受。

## 工具

| 工具 | 参数及结果 |
| --- | --- |
| `emby.instances.list` | 无参数，返回已授权实例列表；不发远端请求 |
| `emby.capabilities` | instance_id；版本、上限和工具名 |
| `emby.users.list` | instance_id；用户 id/name/disabled，不返回用户凭据或策略全文 |
| `emby.libraries.list` | instance_id、可选 user_id；媒体目录或该用户的 Views |
| `emby.items.list` | instance_id、可选 user_id、query；Emby Items/TotalRecordCount |
| `emby.items.get` | instance_id、id、可选 user_id；instance_id/item/version |
| `emby.persons.list` | instance_id、query；分页人物及 ProviderIds |
| `emby.persons.get` | instance_id、name；人物 item/version；更新前仍按人物 ID 调用 items.get |
| `emby.persons.credits` | instance_id、人物 id、query；人物作品关联 |
| `emby.sessions.list` | instance_id、query；完整数组，保留同一条目的多设备会话 |
| `emby.images.list` | instance_id、条目或人物 id；图片类型、索引和标签 |
| `emby.images.download` | instance_id、id、image_type、可选 index、key；封存产物，原图无缩放参数 |
| `emby.metadata.update` | instance_id、id、key、expected_version、fields；保存前值、合并写入和回读核验 |
| `emby.images.upload` | instance_id、id、image_type、可选 index、artifact、key；上传本插件封存图片 |
| `emby.items.refresh` | instance_id、id、key、可选 query；提交刷新 |
| `emby.library.scan` | instance_id、key；提交实例扫描 |
| `emby.media.updated` | instance_id、key、updates；每项 `Path` 与 `UpdateType`（Created/Modified/Deleted） |
| `emby.tasks.list` | instance_id；Emby 计划任务状态、进度及最后结果 |
| `emby.operations.get` | instance_id、operation（上述写工具完整名称）、key；本插件的持久化回执 |

列表 query 使用 Emby 大小写参数：StartIndex、Limit、ParentId、Recursive、SearchTerm、SortBy、SortOrder、Fields、IncludeItemTypes、ExcludeItemTypes、Ids、PersonIds、PersonTypes、Genres、Tags、Years、AnyProviderIdEquals、IsPlayed、IsFavorite、Filters、EnableTotalRecordCount。默认 Limit=100，范围 1–200；逐页检查 TotalRecordCount。查询响应最多 900 KiB，超限后减小分页。会话 query 仅支持 ControllableByUserId、DeviceId、Id。

刷新 query 支持 Recursive、ReplaceAllMetadata、ReplaceAllImages，以及 MetadataRefreshMode / ImageRefreshMode（Default、ValidationOnly、FullRefresh）。媒体变更每次 1–100 条，Path 是 Emby 服务器看到的媒体路径。

## 写入与恢复

所有远端写工具要求确认后的 run 动作；probe、health、migrate 不执行写操作。每次写入提供非空 key（最多 128 字节）。先持久化 pending，再发请求；相同实例、操作、key 和参数复用原回执，参数或连接变化拒绝复用。pending / attention 不自动重新提交，进程重启也保留。SDK 返回回执时，插件必须检查 `state`：completed 才是本次宿主操作完成；attention 需要核验，不把它当成功继续批处理。

扫描、刷新和图片上传的 completed 表示服务器接受 HTTP 请求。扫描是否结束由 tasks.list 或后续 Webhook 判断。Emby 成功的 200 空响应和 204 空响应均被接受；顶层 JSON 数组也受支持。连接中断、回读失败或字段不一致进入 attention。

metadata.update 先读取当前条目并核对 items.get 的 version，只合并 fields 中的字段，保留当前 DTO 里的其他字段，然后 POST 并回读。支持名称/排序名/原名、简介、分级、日期、年份、集号、评分、Genres、Tags、Taglines、Studios、People、ProviderIds、LockedFields、LockData。路径、媒体源、UserData、库配置及条目 ID 不能由此修改。字段类型按工具实现验证，不传整个旧备份 DTO 覆盖新库。

回执 result 保存 before、after、version、accepted、verified。需要撤回时先重新读取条目，使用新的 key 与当前 version，提交选择的 before 字段并验证。宿主串行执行自己的元数据写操作；Emby 公开接口没有原子条件更新合同，与外部编辑器并发修改仍需插件核验和处理。

People 是完整的**所选 People 字段**替换值。插件先读取原名单，按可靠人物标识合并，再把需要保留的 Name/Id/Role/Type/PrimaryImageTag 一并传回。人物自己的 ProviderIds 通过其条目更新。宿主提供人物与作品读写、图片和回执；匹配来源、语言与统一策略由业务插件决定。`physical_person_merge=false`：公开 API 没有专用人物物理合并操作，目标版本的关系重建与作品聚合需实测。

原图单件最多 16 MiB，支持 JPEG/PNG/GIF/WebP/BMP，通过宿主临时流和产物分块存储，图片字节不整体进入 WASM。key 标识不可变版本；图片变化时使用新的 key。读取不使用仪表盘的缩略图链接；响应中的 Token 字段及 URL 凭据被移除。

## 事件

沿用已有每实例 Webhook 地址与通知密钥。验证通过的入库/删除产生 `emby.library.changed`，开始/暂停/恢复/停止产生 `emby.playback.changed`。事件含 instance_id、event、change、status、item_id、item_type、series、season、episode；不转发原始请求、通知密钥或图片鉴权链接。

事件先写入 emby_events.db 的通用 outbox，再回应 Webhook；事件分发、动作去重、停用和任务恢复沿用共用队列。事件按实例隔离，不影响原生通知和剧集合并。源 EventId/时间戳参与去重；旧负载使用 45 秒时间窗，同一负载在跨窗重发时可产生新事件。进度通知仍沿用原生忽略行为。本合同不保证未配置 Webhook 时收到远端变更。

## SDK 与独立开发包

```go
instances, err := pluginwasm.EmbyInstances()
if err != nil { return pluginapi.Result{}, err }
// instanceID 由用户从已授权列表选择；itemID 来自分页查询。
item, err := pluginwasm.EmbyGetItem(instanceID, itemID, "")
if err != nil { return pluginapi.Result{}, err }
receipt, err := pluginwasm.EmbyUpdateMetadata(pluginapi.EmbyInput{
    InstanceID: instanceID, ID: itemID, Key: operationKey,
    ExpectedVersion: item.Version,
    Fields: map[string]any{"Overview": overview},
})
if err != nil { return pluginapi.Result{}, err }
return pluginapi.Result{Data: map[string]any{"receipt": receipt, "instances": instances}}, nil
```

`python tools/plugin-kit.py --out OUTPUT_DIR --id my-emby-tools --emby` 导出只读合同骨架和完整 SDK，不安装业务插件。需要写入时自行声明最小写权限、danger 与 confirmation，并处理回执。

媒体元数据备份插件可以分页导出条目、人物和原图，用现有 artifacts / files.publish 保存 JSON、NFO 和图片。新库恢复先按路径、类型、供应商标识建立新 ID 映射，再选择字段和图片恢复、通知文件变更或扫描。完整 Emby ProgramData / 数据库一致性备份和大型归档属于另一类来源合同，本工具集不将 HTTP 元数据导出声明为完整应用备份。

Emby 能力仅通过插件 SDK 调用。插件在插件管理中选择实例和工具；外部 API 设置沿用现有网盘文件权限。

协议依据：[Emby API Key](https://dev.emby.media/doc/restapi/API-Key-Authentication.html)、[Items 查询](https://dev.emby.media/reference/RestAPI/ItemsService.html)、[人物](https://dev.emby.media/reference/RestAPI/PersonsService.html)、[条目更新](https://dev.emby.media/reference/RestAPI/ItemUpdateService.html)、[图片](https://dev.emby.media/reference/RestAPI/ImageService.html)、[媒体库](https://dev.emby.media/reference/RestAPI/LibraryService.html)、[刷新](https://dev.emby.media/reference/RestAPI/ItemRefreshService.html)、[会话](https://dev.emby.media/reference/RestAPI/SessionsService.html)。
