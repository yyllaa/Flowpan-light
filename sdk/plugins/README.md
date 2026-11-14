# Flowpan 外部插件开发包

此目录可独立开发，SDK 与合同来自导出时的项目源码，仅依赖 Go 标准库。

`definition.network.proxy` 声明网络策略：direct 为直连，project 必须使用项目设置中的 HTTP 代理，inherit 跟随项目设置。代理地址由宿主管理。导出时可用 --proxy project 选择需要代理的插件。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

生成的 ZIP 仅包含清单和 WASM，SDK 源码不会进入运行包。构建与校验不执行插件。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
使用 --storage 导出网盘驱动模板：安装启用后在项目账号设置、文件浏览、STRM、上传、整理、挂载和有声书中使用。
网盘模板通过独立的 Storage 处理函数接收账号上下文；按目标网盘改写协议与 http_hosts。模板的 HTTPS JSON 接口只是示例，默认没有配置任何账号。
SDK 的 Serve 分开处理 run/health/migrate/storage。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，也不需要独立服务镜像。资源插件通过项目任务操作网盘；网盘驱动插件实现自身声明的文件协议。


## 后续插件通用接入

新版开发包随清单生成 release.notes 与 CHANGELOG.md，默认执行 pack --release 和 check --release。宿主从声明自动接入更新提示、仪表盘及 FUSE，SDK 通过 pluginapi.StorageError 保留网盘错误，不增加品牌分支。完整字段、兼容规则与验收方法见 [插件通用接入合同](../../docs/plugin-integration-contract.md)。发布校验与新元数据需要支持该合同的宿主。
