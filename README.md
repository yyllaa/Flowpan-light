# Flowpan-light

Flowpan-light 是面向 **115 网盘与中国移动云盘（139）** 的媒体管理服务，提供多账号文件浏览、转存下载、自动整理、STRM、云盘挂载、有声书和播放代理。也可通过外部插件接入联通云盘、OpenList 和其他能力。

## 快速入口

| 想做什么 | 去哪里 |
| --- | --- |
| 在 Docker 或 NAS 上部署 | [部署指南](docs/deployment.md) |
| 配置账号、STRM、挂载与有声书 | [使用指南](docs/使用指南.md) |
| 更新主程序、备份数据或回退镜像 | [升级与维护](docs/maintenance.md) |
| 更新或回退后台 Web 界面 | [Web 更新指南](docs/web-update.md) |
| 安装、更新和管理插件 | [插件使用](docs/plugins.md) · [插件目录](plugins/README.md) |
| 查看外部 API、插件开发合同和 SDK | [文档目录](docs/README.md) |
| 下载 Web 更新包、插件与开发包 | [GitHub Releases](https://github.com/yyllaa/Flowpan-light/releases) |

主程序通过预构建的离线镜像交付。部署配置在 [deploy/](deploy/)，重建容器时沿用原来的 `/config` 映射。

## 仓库目录

- [docs/](docs/) — 用户指南、开发合同与历史记录。
- [deploy/](deploy/) — Compose 配置与环境变量示例。
- [plugins/](plugins/) — 插件市场目录、插件源码与示例。
- [sdk/plugins/](sdk/plugins/) — 可独立使用的插件 SDK 与合同。
- [tools/](tools/) — 插件模板导出与发布检查工具。
