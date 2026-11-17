# Flowpan 外部插件目录

在“辅助插件 → 插件管理 → 插件源”添加：

    https://github.com/yyllaa/Flowpan-light/tree/main/plugins

刷新后在插件市场选择需要的插件。添加来源不会自动安装或启用插件。

| 插件 | 稳定 ID | 版本 | 独立目录 | 网络 |
| --- | --- | --- | --- | --- |
| 癫影（浏览器版） | resource-dian | 1.1.1 | [dian](dian) | 沿用项目设置 |
| 癫影 OpenAPI | resource-dian-openapi | 1.3.10 | [dian-openapi](dian-openapi) | 必须使用项目 HTTP 代理 |
| 联通云盘 | storage-wopan | 0.2.2 | [wopan](wopan) | 直连 |
| 115 ED2K | ed2k | 1.0.2 | [ed2k](ed2k) | 由宿主读取授权文件 |
| 音乐刮削 | music-scraper | 1.0.0 | [music](music) | 使用项目代理和各音源间隔 |

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

[下载插件与 SDK](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.16-ed2k)。

联通云盘 0.2.2 修复可写 FUSE 的删除与移动，需要包含移动后引用及 inode 同步修复的主程序。

[下载联通云盘 0.2.2 修复包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.14-wopan-fuse)。

115 ED2K 1.0.0 与癫影 OpenAPI 1.3.9 需要 `2026.11.16-ed2k` 或包含同等合同的主程序。ED2K 从文件浏览勾选生成，在导航集中管理；癫影离线发布可筛选已生成结果。首次生成读取完整文件，默认串行，重复结果复用缓存。详见 [ED2K 工具与扩展合同](../docs/plugin-ed2k.md)。

115 ED2K 1.0.1 优化手机布局、按钮和记录密度，配套主程序 `2026.11.16-ed2k-ui`。安装与本版说明见 [ED2K](ed2k/README.md) 和 [更新日志](ed2k/CHANGELOG.md)。

癫影 OpenAPI 1.3.10 修复分享结果中的 TMDB 编号显示；115 ED2K 1.0.2 收紧管理页留白并接入任务中心。配套主程序为 `2026.11.16-plugins-mobile-tmdb`，先更新主程序，再刷新已有插件源并检查更新。详见各插件更新日志。


音乐刮削需要主程序 `2026.11.17-music-runtime` 或包含相同合同的后续版本。先更新主程序，再刷新插件源。详见 [音乐插件](music/README.md) 和 [宿主运行合同](../docs/plugin-music-runtime.md)。

插件依赖、可选功能、兼容性预检和自定义扩展见[声明式兼容合同](../docs/plugin-compatibility.md)。音乐刮削 1.0.1 使用此合同，需要 `2026.11.17-plugin-contract` 或具备相同能力的宿主。

音乐刮削 1.0.2 同步新版独立 SDK，保留原音源、请求间隔、缓存、定时和校验回写流程。新版 SDK、能力提供者/调用方开发包及合同见 [本次发布](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.17-providers)。自主能力层配套主程序 `2026.11.17-plugin-providers`；音乐插件自身继续按清单中实际需要的功能检查兼容性。

音乐刮削 1.0.3 修复来源只显示 115 的问题，支持选择已授权网盘及本机目录，目录浏览复用项目控件。先更新主程序到 `2026.11.17-plugin-file-picker` 或满足 `ui.file-picker: 1` 的版本。
