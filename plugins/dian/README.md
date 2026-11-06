# 癫影外部插件 1.1.1

癫影提供账号连接检查、每日签到、影视资源搜索和确认解锁，通过 Flowpan 的通用浏览器工具运行。

## 安装

1. 使用支持外部 WASM 插件和浏览器工具的新版 Flowpan，在“其他设置 → 浏览器维护”下载浏览器。
2. 在“辅助插件 → 插件管理 → 插件源”手动添加 `https://github.com/yyllaa/Flowpan-light` 并刷新，选择癫影安装；也可从 [发布页](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06) 下载 `resource-dian-1.1.1.zip` 后本地导入。
3. 填写自己的癫影账号和密码并启用。首次安装默认停用，定时签到和通知默认关闭。
4. 搜索并选择资源，确认解锁后通过项目“转存 / 下载”入口衔接既有转存、整理流程。

浏览器按插件与账号隔离登录状态。签到按上海时区每日保存成功回执，已解锁资源复用回执；结果待核验时保留状态。账号和密码由用户在宿主中填写，源码和下载包只提供配置字段。

## 开发

需要 Go 1.25+ 与 Python 3。在仓库根目录运行 `python plugins/build.py --out dist/plugins`；单独执行测试时在本目录运行 `go test ./...`。`go.mod` 通过相对路径引用 `../../sdk/plugins`，协议使用宿主 ABI v1。

本次版本包括七位数 TMDB ID 的数字解析修复。来源说明见 [NOTICE.md](NOTICE.md)。
