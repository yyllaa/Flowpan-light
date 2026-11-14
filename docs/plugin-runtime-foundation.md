# 插件运行基础

本合同提供未来插件可组合调用的宿主能力，不提供音乐刮削、字幕生成或分享站点等业务插件。已有整理、STRM、下载、识别入口、配置和自动触发条件保持原样；新入口复用相应引擎。ABI 仍为 v1，旧参数默认行为保持不变。工具、权限、输入结构及上限以 `host.capabilities` 和 `plugin-host-tools.json` 为准。

## 能力与 SDK

| 基础能力 | 宿主工具 | SDK 与行为 |
| --- | --- | --- |
| 生成字节 | artifacts.create/write/seal/stat/read/release | PutArtifact 自动按 32 KiB 写入并校验 SHA-256；封存后不可变 |
| 文件输入与中转 | files.read/stage/publish/task/cancel/release/restore | 云盘和授权本机文件；大文件由宿主搬运，不放进 WASM 内存 |
| 结构化文件管理 | files.mkdir/rename/move/copy/delete | MutateFiles；按来源 capabilities 检查，同来源批量操作，跨来源用暂存和回写 |
| 影视识别 | media.identify | IdentifyMedia；复用文件名、季集解析，可显式选择项目 AI 与 TMDB |
| 独立文件整理 | organize.preview/apply | PreviewOrganize/ApplyOrganize；插件传文件清单和目标，调用现有命名模板 |
| 文本 AI | ai.status/complete | AIStatus/CompleteAI；宿主持有服务凭据、模型、代理、限流和备用服务 |
| 外部处理服务 | service.upload | ServiceUpload；宿主直接把媒体输出、暂存文件或封存产物发到插件声明的辅助服务 |
| 媒体处理 | media.probe/submit/task/result/cancel/release | 分段音频、单声道及采样率；MKV 可选择保留所有轨道和章节 |
| 文件描述 | cas.inspect/create、strm.inspect/create | 兼容 CAS 描述；外部播放地址生成独立 STRM 产物，不创建原生 STRM 任务 |
| 下载器 | downloads.list/status/add/action | 复用项目已保存下载器、保存目录、分类与认证，不向插件返回连接凭据 |
| 依赖 | dependencies.status/ensure | 复用 FFmpeg、FFprobe、浏览器的安装状态、已配置下载源和在途安装 |
| 输出下载分享 | shares.create/revoke | 宿主输出的限时 GET/HEAD、Range 下载；插件可先自行生成密文产物 |

私有 KV、进度、后台任务、定时、取消、重试、文件变更事件、浏览器、受声明主机约束的 HTTP、通知、页面及仪表盘计数继续复用既有合同。文件 ID、账号 ID、游标为不透明字符串；相同显示名称不表示相同身份。每次调用重新检查当前安装版本、权限、工具开关与提交时授权交集。

单次 WASM 动作仍受 128 次宿主调用与 64 MiB 内存上限约束。PutArtifact 适合字幕、清单等小产物；较大的生成内容分多次动作使用 CreateArtifact/WriteArtifact/SealArtifact，并保存 Written 偏移继续。64 MiB 是单产物总量上限，不是单次 WASM 可一次性传入的大小。

## 产物与文件流

`artifacts.write` 权限允许创建、追加、封存和释放；读取及消费产物需要 `artifacts.read`。单个产物最多 64 MiB，单插件预留总计 256 MiB、记录最多 1024 条。写入必须连续；相同偏移仅允许完全相同的重试。SDK PutArtifact 把内容 SHA-256 绑定到幂等键，同键同长度的不同内容也被拒绝。封存核对预期大小和摘要，更新包或停用后拒绝旧输出的新消费。

```go
artifact, err := pluginwasm.PutArtifact(key, name, generatedBytes)
// 检查 err 后，将 artifact.ID 存入插件自己的检查点。
task, err := pluginwasm.PublishFile(pluginapi.FilePublish{
    Key: publishKey, Account: accountID, ParentID: outputID, Name: name,
    Artifact: artifact.ID, Conflict: "keep_both",
})
// 下一次动作查询 task.ID；使用返回的 Result.ID，而非猜测最终名称。
```

files.publish 的 media_task/artifact/file_task 三选一。文件上传消费期间固定输入并阻止释放；取消、释放、回写和原文件恢复沿用原合同。本机暂存与生成输出同样可以回写云盘；跨账号中转不要求先运行媒体处理。替换原文件继续要求对应媒体暂存输入，保留原始备份，不隐式覆盖。

## 外部 STRM 与本地视频整理

strm.create 接收 `key/name/url`，返回普通封存产物。播放协议支持 http、https、smb、nfs、rtsp，文件包含单个 URL。宿主不访问播放地址；分享站点的文件枚举、提取码或播放链接解析由将来的插件通过对应驱动或站点协议完成。strm.inspect 读取已授权文件并返回原链接、大小及 SHA-256。

organize.preview 接收 `key/account/ids/output_id`，当前 account 为 local，单次 1–100 个外部 STRM 或本地影视文件。AI、TMDB 为可选布尔值；AI 还需 ai.use。metadata 可按文件 ID 提供 title/year/type/tmdb_id/season/episode，供插件人工校正或使用自己的识别结果。未给元数据时复用项目解析，命名复用项目已保存模板。输出包含文件清单、生成名称、目标 ID、冲突和预览 ID，不包含 STRM 的播放 URL。预览不改输入、不创建媒体目录，私有计划保留 24 小时、单插件最多 100 个。

```go
plan, err := pluginwasm.PreviewOrganize(pluginapi.OrganizePreview{
    Key: previewKey, Account: "local", IDs: selectedFiles, OutputID: libraryRoot,
    AI: false, TMDB: false,
})
// 检查 err，展示 plan.Items；已确认的 run 动作使用稳定幂等键执行。
err = pluginwasm.ApplyOrganize(plan.ID, applyKey, &result)
```

apply 需要 organize.write 和已确认动作。执行前复核全部来源快照及 STRM 字节摘要；同名冲突跳过、不覆盖，幂等回执阻止重复移动。文件按 Movies/TV、模板目录及季目录归类；扩展名和原始字节保留。同文件系统优先使用不覆盖的硬链接，跨授权根或不支持硬链接时流式复制，核对大小后移除旧路径，原始保留副本路径随结果返回。中途失败记录 attention，不自动重复未知写入。大型跨盘复制受本次动作超时约束，原始内容保留；插件可拆批并查询自己的任务回执。

这条调用链不读取项目生成 STRM 的输出账本，也不启动 organize.run 或修改现有任务。strm.tasks/run/status 和 organize.run/status/stop 是调用原生任务的另一组入口，参数语义保持原样。云盘文件可通过 media.identify 配合结构化文件管理，或使用原生整理任务。

## AI、字幕与辅助服务

ai.complete 是项目现有文本模型的通用请求，支持 chat_completions 和 responses、JSON 输出、配置中的代理与备用服务。ai.status 不返回地址或密钥。文本输入与 system 合计最多 48 KiB，输出上限 8192 tokens。音频识别引擎通过辅助服务接入，不把文本模型声称为语音识别模型。

media.submit 的 transcode 可传 `start_seconds/duration_seconds/audio_channels/sample_rate`；例如分段输出 16 kHz 单声道 WAV 给语音服务。remux 为 MKV 时可传 `map_all:true` 保留所有轨道和章节。旧请求不传这些字段时继续沿用原有选流行为；目录原盘和 ISO 的播放列表提取需要对应解析流程。

service.upload 与 service.call 复用 definition.service 的 URL、Token、credentials，宿主发送 POST multipart，包含 metadata JSON 字段和默认名为 file 的文件字段。可通过 field 改固定字段名；metadata 由宿主绑定 credentials/read_only/confirmed/client_id。input 的 media_task/file_task/artifact 三选一，单次最多 64 MiB；不跟随重定向、不自动重试未知请求，凭据回显被隐藏。服务端需消费完整文件后返回有界 JSON；大型字幕任务可返回服务任务 ID，再通过 service.call 查询。

```go
err := pluginwasm.ServiceUpload(pluginapi.ServiceUpload{
    Path: "/transcribe", Input: pluginapi.HostInput{MediaTask: audioTaskID},
    Field: "file", Body: map[string]any{"language":"zh"},
}, &serviceResult)
```

## 下载器、依赖、CAS 与分享

downloads.status 默认 filter 为 all，支持项目原生的 all/downloading/seeding/completed/active/inactive/errored。downloads.add 使用已保存目录、分类、标签；action 仅接受 pause/resume/recheck/reannounce 与合法任务哈希。写操作需要已确认动作和稳定幂等键。dependencies.ensure 只接受 ffmpeg/ffprobe/browser，沿用项目下载源和安装器，不执行插件指定程序或包管理命令。所有依赖仍按需安装。

cas.create 基于已完成暂存的实际名称、大小及 SHA-256 创建项目兼容 JSON 描述，cas.inspect 复用原有解析。描述不等同于所有网盘均具备秒传恢复；恢复能力取决于来源实际实现。

shares.create 接收 key、input 及 expires_seconds，返回宿主相对 path、ID、到期时间。最长 24 小时，默认 1 小时。访问不需要管理员会话，以随机令牌和当前插件版本、权限及来源授权验证；支持 GET/HEAD/Range。停用、更新、撤销、到期、撤销目录或释放输入后失效。链接不会永久占用输入；长时间传输逐次复核。分享是宿主输出下载通道，不在每次下载时执行自定义 WASM 代理。插件可以使用标准库生成密文产物，再把该产物交给同一分享接口。

## 后续开发约定

只声明实际使用的权限，先查询能力再组合 SDK。业务逻辑、特定站点协议、刮削匹配、音频模型和同步冲突规则由插件实现；宿主负责字节传输、依赖、权限、任务和运行状态。未来能力按通用合同扩展，配套 SDK、目录、自动检查和独立回滚验证，不增加插件品牌或 ID 分支。
