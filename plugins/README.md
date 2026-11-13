# Flowpan 外部插件目录

在“辅助插件 → 插件管理 → 插件源”添加：

    https://github.com/yyllaa/Flowpan-light/tree/main/plugins

刷新后在插件市场选择需要的插件。添加来源不会自动安装或启用插件。

| 插件 | 稳定 ID | 版本 | 独立目录 | 网络 |
| --- | --- | --- | --- | --- |
| 癫影（浏览器版） | resource-dian | 1.1.1 | [dian](dian) | 沿用项目设置 |
| 癫影 OpenAPI | resource-dian-openapi | 1.3.7 | [dian-openapi](dian-openapi) | 必须使用项目 HTTP 代理 |
| 联通云盘 | storage-wopan | 0.2.1 | [wopan](wopan) | 直连 |

两个癫影版本使用不同 ID，各自安装、配置、定时任务、运行记录和私有数据，互不覆盖。浏览器版需要项目浏览器能力，OpenAPI 版通过宿主 sdk.call 使用私有 SDK，无需浏览器。

联通云盘安装启用后在原生账号中心扫码添加账号，进入文件浏览、STRM、挂载、有声书和上传入口。文件操作范围仍由宿主管理。

新版 OpenAPI 和联通插件需要支持网络声明、SDK 桥接和网盘驱动合同的配套新版宿主。先更新主程序，再安装插件；插件包不包含主程序、浏览器或 FFmpeg。

## 目录与构建

每个插件在 plugins 下有独立目录、manifest.json 和 go.mod；SDK 位于 [sdk/plugins](../sdk/plugins)。新插件加入 plugins/index.json 后，用户刷新同一插件源即可选择安装。

需要 Go 1.25+ 和 Python 3。在仓库根目录运行：

    python plugins/build.py --out dist/plugins

只构建部分插件：

    python plugins/build.py --plugin dian-openapi --plugin wopan --out dist/selected

在各插件目录运行 go test ./...。构建器从清单生成独立 WASM ZIP，并输出 SHA-256；可用 --host 指定 Flowpan 可执行文件校验。

索引固定同仓库 Release 下载地址、SHA-256 和源码 commit。插件配置与凭据由用户在宿主中填写，公开源码与发布包不包含运行数据。

[下载插件与 SDK](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.13)。
