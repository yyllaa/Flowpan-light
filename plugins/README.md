# Flowpan 外部插件目录

## 安装和更新

在“辅助插件 → 插件管理 → 插件源”添加：

    https://github.com/yyllaa/Flowpan-light/tree/main/plugins

刷新后在插件市场选择需要的插件。添加来源不会自动安装或启用插件；已有插件可在刷新来源后检查更新。

| 插件 | 稳定 ID | 版本 | 独立目录 | 网络 |
| --- | --- | --- | --- | --- |
| 癫影（浏览器版） | resource-dian | 1.1.1 | [dian](dian) | 沿用项目设置 |
| 癫影 OpenAPI | resource-dian-openapi | 1.3.11 | [dian-openapi](dian-openapi) | 必须使用项目 HTTP 代理 |
| 联通云盘 | storage-wopan | 0.2.3 | [wopan](wopan) | 直连 |
| 115 ED2K | ed2k | 1.0.3 | [ed2k](ed2k) | 由宿主读取授权文件 |

两个癫影版本使用不同 ID，各自安装、配置、定时任务、运行记录和私有数据，互不覆盖。浏览器版需要项目浏览器能力，OpenAPI 版通过宿主 sdk.call 使用私有 SDK，无需浏览器。

联通云盘安装启用后在原生账号中心扫码添加账号，进入文件浏览、STRM、挂载、有声书和上传入口。文件操作范围仍由宿主管理。115 ED2K 从文件浏览勾选生成，在导航集中管理；首次生成读取完整文件，默认串行，重复结果复用缓存。详见 [ED2K 工具与扩展合同](../docs/plugin-ed2k.md)。

插件独立发布；安装或更新时请检查插件声明的宿主能力与兼容性要求。OpenAPI 和联通云盘需要宿主支持网络声明、SDK 桥接和网盘驱动合同；宿主缺少所需能力时先更新主程序。插件包不包含主程序、浏览器或 FFmpeg。依赖、可选功能、兼容性预检和自定义扩展见[声明式兼容合同](../docs/plugin-compatibility.md)。

## 下载与发布

- [当前 OpenAPI、联通云盘和 ED2K 插件包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.17-fixes)；[发布说明](../RELEASE-PLUGINS-2026.11.17-FIXES.md)。
- [癫影浏览器版 1.1.1 插件包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06)。
- [SDK 与能力提供者开发包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.17-providers)；自主能力层需要配套主程序 2026.11.17-plugin-providers 或具备相应能力的版本。
- [历史发布](https://github.com/yyllaa/Flowpan-light/releases)保留旧版本及配套要求。GitHub 的 Latest 标记指向一个 SDK 发布，不代表其中包含所有最新插件；插件市场以 [index.json](index.json) 为准。

## 目录与构建

每个插件在 plugins 下有独立目录、manifest.json 和 go.mod；SDK 位于 [sdk/plugins](../sdk/plugins)。新插件加入 plugins/index.json 后，用户刷新同一插件源即可选择安装。

需要 Go 1.25+ 和 Python 3。在仓库根目录运行：

    python plugins/build.py --out dist/plugins

只构建部分插件：

    python plugins/build.py --plugin dian-openapi --plugin wopan --out dist/selected

在各插件目录运行 go test ./...。构建器从清单生成独立 WASM ZIP，并输出 SHA-256；可用 --host 指定 Flowpan 可执行文件校验。

索引固定同仓库 Release 下载地址、SHA-256 和源码 commit。插件配置与凭据由用户在宿主中填写，公开源码与发布包不包含运行数据。
