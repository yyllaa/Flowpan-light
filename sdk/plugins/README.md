# Flowpan 外部插件开发包

此目录可独立开发，SDK 与合同来自导出时的项目源码，仅依赖 Go 标准库。

`definition.network.proxy` 声明网络策略：direct 为直连，project 必须使用项目设置中的 HTTP 代理，inherit 跟随项目设置。代理地址由宿主管理。导出时可用 --proxy project 选择需要代理的插件。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限，并填写 release.notes 与 CHANGELOG.md。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

发布版本使用数字版本号，支持 v 前缀、预发布及日期版本；新版本同时更新清单的两层 version、release.notes 与 CHANGELOG.md。build.py 自动运行 pack --release 和 check --release，缺少更新日志或版本不可比较时终止发布；普通 check 继续兼容旧包。
release 可声明 title、notes、published_at（RFC3339）和 url（HTTPS），notes 随安装包保存，本地导入和任意发布站点均可阅读。旧 GitHub 包继续自动定位 Release。
HTTP、SDK 和服务请求由宿主自动计入仪表盘；网盘提供者与账号从运行上下文绑定，不需要新增仪表盘页面或为品牌注册计数。
网盘返回 pluginapi.StorageError 时，SDK 自动保留业务码和 HTTP 状态，FUSE 使用实际提供者标签；只读与可写功能按 storage.capabilities 自动适配。

生成的 ZIP 包含清单、WASM 和可选的 ui/ 静态前端；宿主私有 SDK 不会进入运行包。构建与校验不执行插件。
使用 --page 导出独立页面示例，包含可分别维护的 HTML、CSS、ES 模块和页面桥接 SDK。
页面通过 FlowpanPage.connect() 调用自己声明的动作和任务；凭据与媒体工具保留在 WASM/宿主一侧。
音乐示例使用 media.submit 的 remux 操作保留编码并写入常用标签，输入路径必须处于授权目录，输出进入插件私有媒体目录。
使用 --files 导出云盘文件处理示例：files.stage → media.probe / media.submit(file_task) → files.publish，包含进度、取消、释放和恢复动作。每次动作只提交或查询一步，大文件通过宿主传输。
files.capabilities 返回账号支持的冲突策略。replace 为备份后替换，非原子操作；原文件保留为备份，可用 files.restore 恢复。attention 表示回执待核验，同一任务键不会自动重复写入。此示例填写标签，不包含音乐网站的匹配协议；FFmpeg/FFprobe 仍从项目依赖下载入口安装。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
SDK 同时包含通用运行基础：PutArtifact、ServiceUpload、CompleteAI、IdentifyMedia、PreviewOrganize/ApplyOrganize、CreateSTRM/InspectSTRM、CreateCAS、下载器与依赖、ShareOutput。先调用 Capabilities 核对实际宿主；按方法声明权限。独立整理只使用插件传入的文件清单，不修改项目原生整理或 STRM 配置。files.publish 的 media_task/artifact/file_task 三选一；使用返回的 result.id 定位实际输出。
使用 --emby 导出 Emby 实例与条目查询的只读宿主合同骨架；授权在插件管理中选择实例。EMBY.md 包含分页、元数据、图片、回执、事件和外部 API 合同。
使用 --interaction 导出通用机器人交互骨架。启用后发送 /plugins 查看入口；/plugin 插件ID 命令 始终定位当前插件，唯一命令也可直接用 /命令。SDK 提供 InteractionContext、ReplyInteraction、WaitInteraction、EndInteraction、StageInteractionAttachment、SendInteractionFile。附件为插件私有产物，可交给 media.probe/submit 的 artifact 输入；Telegram 使用原生按钮，企业微信使用 /reply 回调入口。会话按通道、用户、聊天和话题隔离，危险动作由宿主再次确认。合同见 docs/plugin-interaction.md。
使用 --storage 导出网盘驱动模板：安装启用后在项目账号设置、文件浏览、STRM、上传、整理、挂载和有声书中使用。
网盘模板通过独立的 Storage 处理函数接收账号上下文；按目标网盘改写协议与 http_hosts。模板的 HTTPS JSON 接口只是示例，默认没有配置任何账号。
使用 --recognition 导出新识别来源的合同骨架；实现 Recognition 处理函数，通过 RecognitionReply 返回身份或待确认状态。RECOGNITION.md 说明关键词自动接入原整理链路与分类目录绑定。
SDK 的 Serve 分开处理 run/health/migrate/storage/recognition。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，也不需要独立服务镜像。资源插件通过项目任务操作网盘；网盘驱动插件实现自身声明的文件协议。


## ED2K 与文件浏览扩展

本 SDK 提供 `GenerateED2K`、`ListED2K`、`ED2KTask`、`RetryED2K`、`CancelED2K` 和 `RemoveED2K`，与 `ed2k.generate/read/manage` 权限配合。文件选择入口、提供者过滤、上下文动作及动态多选见 [ED2K 合同](../../docs/plugin-ed2k.md)。宿主需要 `2026.11.16-ed2k` 或包含同等合同的版本。

## 声明式兼容

使用清单 `requires` / `optional` 声明依赖，`extensions` 保存带命名空间的自定义数据。运行时用 `pluginwasm.CheckRequirements` 查询必需与可选缺项。参见[兼容合同](COMPATIBILITY.md)。

## 自主提供与调用能力

`definition.provides` / `definition.uses` 声明版本化业务接口和必需、可选依赖；SDK 的 `CallCapability`、`SubmitCapability`、`GetCapabilityTask`、`CancelCapabilityTask` 复用宿主执行和任务中心。唯一可用提供者自动绑定，多候选在已有授权弹窗选择，重启保留绑定。

完整示例见 [能力合同](CAPABILITIES.md)。仓库根目录可运行：

```powershell
python tools/plugin-kit.py --out output/format-provider --id format-provider --capability-provider
python tools/plugin-kit.py --out output/format-consumer --id format-consumer --capability-consumer
```

生成器从本目录复制 SDK，无需主程序源码。先启用提供者，再启用调用方。新合同需要 `plugins.capabilities: 1`；声明增加业务接口，新增宿主原语仍需宿主实现。带源代码的两个独立开发包同时随本次 Release 提供。

来源与目录选择声明见 [FILE-PICKER.md](FILE-PICKER.md)：`account-list` 可声明 `max: 1`，`path` 可通过 `extensions.flowpan.files.account_field` 绑定来源，复用宿主目录选择器。需要宿主 `ui.file-picker: 1`。
