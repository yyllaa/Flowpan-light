# Flowpan 外部插件开发包

此目录可独立开发，SDK 与合同来自导出时的项目源码，仅依赖 Go 标准库。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

生成的 ZIP 仅包含清单和 WASM，SDK 源码不会进入运行包。构建与校验不执行插件。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
SDK 的 Serve 分开处理 run/health/migrate。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，不直接操作云盘，也不需要独立服务镜像。
