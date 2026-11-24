# Flowpan 外部插件目录

在“辅助插件 → 插件管理 → 插件源”添加下列地址。完整步骤见[插件安装和更新](../docs/plugins.md)。

```text
https://github.com/yyllaa/Flowpan-light/tree/main/plugins
```

| 插件 | 稳定 ID | 版本 | 发布 |
| --- | --- | --- | --- |
| [癫影（浏览器版）](dian/README.md) | resource-dian | 1.1.1 | [下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06) |
| [癫影 OpenAPI](dian-openapi/README.md) | resource-dian-openapi | 1.3.12 | [下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.19-dian) |
| [联通云盘](wopan/README.md) | storage-wopan | 0.2.5 | [下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.22-wopan-sdk) |
| [115 ED2K](ed2k/README.md) | ed2k | 1.0.3 | [下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.17-fixes) |
| [OpenList](openlist/README.md) | storage-openlist | 0.1.1 | [下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.24-openlist-0.1.1) |

插件市场以 [index.json](index.json) 为准。各插件目录说明配置、网络和配套主程序要求。其他版本及当时的验收记录见[发布归档](../docs/archive/README.md)。

## 开发与构建

各插件有独立目录、`manifest.json` 和 `go.mod`。开发入口见[插件开发指南](../docs/plugin-development.md)和[独立 SDK](../sdk/plugins/README.md)。

- [SDK 开发包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.24-openlist)
- [能力提供者与调用方示例包](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.17-providers)
- [固定宿主发布验收](../sdk/plugins/RELEASE.md)

需要 Go 1.25+ 和 Python 3。以下命令在仓库根目录运行，可用 `--plugin` 指定插件目录名。

```bash
python plugins/build.py --out dist/plugins
python plugins/build.py --plugin dian-openapi --plugin wopan --out dist/selected
```

构建器产出独立 WASM ZIP 和 SHA-256。使用 `--host` 指定 Flowpan 可执行文件进行校验。插件示例在 `plugins/examples/` 使用独立 Go 模块，随模块测试一并检查。
