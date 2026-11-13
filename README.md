# Flowpan-light

Flowpan-light 是面向 **115 网盘与中国移动云盘（139）** 的媒体管理服务，提供多账号文件浏览、转存下载、自动整理、STRM、云盘挂载、有声书和播放代理，并支持通过外部插件扩展资源查询等能力。

本仓库提供项目文档、Docker 部署示例、外部 API / SDK 与插件目录。**主程序通过预构建镜像交付，部署不需要主程序源码。**

## 功能

| 功能 | 说明 |
| --- | --- |
| 账号与文件管理 | 115 / 139 多账号，按账号浏览文件、选择目录和管理传输任务。 |
| 转存下载 | 分享链接转存、文件与文件夹传输；115 跨账号复制使用秒传链路，命中情况取决于云盘返回。 |
| 自动整理 | 媒体识别、分类、命名、洗版策略与整理日志，可衔接转存流程。 |
| STRM | 115 / 139 来源目录绑定、首次同步与增量更新、附属文件处理。 |
| 云盘挂载 | Linux / NAS 通过 FUSE 挂载所选目录，多账号统一使用 `/cloud` 父目录。 |
| 有声书 | 本地与网盘书库、章节索引、元数据管理和播放；每个网盘书库独立同步。 |
| 播放代理 | Emby 代理与飞牛影视接入，按页面配置播放和网络策略。 |
| 扩展与集成 | 通知互动、115 / 139 外部 API、外部 WASM 插件和共用宿主工具。 |

**FFmpeg / FFprobe 和浏览器均在“其他设置”中按需下载、更新和切换下载源，不随主程序镜像预装。** 插件调用项目提供的浏览器、网络、转存、整理等能力。

## Docker 快速部署

准备 Docker Engine 和 Docker Compose，以及与服务器架构匹配的 **Flowpan 主程序离线镜像包**。目前本仓库 [Releases](https://github.com/yyllaa/Flowpan-light/releases) 发布的是插件与 SDK；主程序镜像包按单独交付的文件使用。

在服务器新建部署目录，下载配置：

```bash
mkdir -p flowpan/deploy
cd flowpan
curl -fL https://raw.githubusercontent.com/yyllaa/Flowpan-light/main/deploy/compose.yaml -o deploy/compose.yaml
curl -fL https://raw.githubusercontent.com/yyllaa/Flowpan-light/main/deploy/.env.example -o .env
```

导入取得的主程序镜像包，将下面的路径换成实际文件路径：

```bash
docker load -i /path/to/flowpan-image.tar.gz
```

将 `.env` 中的 `FLOWPAN_IMAGE` 改为命令输出 `Loaded image:` 后的完整镜像标签。示例中的 `flowpan-light-core:installed` 只是本地标签占位值，必须替换为已导入的标签。然后启动：

```bash
mkdir -p config logs media
docker compose --env-file .env -f deploy/compose.yaml config --quiet
docker compose --env-file .env -f deploy/compose.yaml up -d --pull never
docker compose --env-file .env -f deploy/compose.yaml logs --tail=100
```

浏览器访问 `http://服务器IP:8080`，首次打开时创建管理员账号。端口可在 `.env` 中修改。

配置文件路径固定为 `deploy/compose.yaml`，其中相对目录映射到同一部署目录下的 `config/`、`logs/`、`media/`；请保留这套目录结构。完整步骤、NAS 配置和可选 FUSE 挂载见 [部署指南](docs/deployment.md)。

## 首次使用

1. 在账号中心添加 115 或 139 账号，扫码并检查登录状态。
2. 在“其他设置”按需保存网络代理、GitHub Token、TMDB、通知等配置。
3. 使用音频探测或浏览器插件前，在对应设置中下载 FFmpeg 或浏览器并确认就绪。
4. 在 STRM、有声书、自动整理等页面分别绑定要处理的账号和目录，再执行首次同步或任务。
5. 使用插件时，先手动添加插件源，在市场选择安装，再配置账号和授权并启用。

## 外部插件

在“辅助插件 → 插件管理 → 插件源”手动添加：

```text
https://github.com/yyllaa/Flowpan-light/tree/main/plugins
```

刷新来源后，在插件市场选择需要的插件。添加来源只读取目录，不会批量安装或自动启用插件。当前提供 [癫影浏览器版](plugins/dian/README.md)、[癫影 OpenAPI](plugins/dian-openapi/README.md) 和 [联通云盘](plugins/wopan/README.md)，各插件独立安装与维护。

- [插件安装、更新、卸载与数据](docs/plugins.md)
- [插件目录](plugins/README.md)
- [插件开发与 SDK](docs/plugin-development.md) · [宿主工具](docs/plugin-host-tools.md) · [独立 SDK](sdk/plugins/README.md)

## 文档

| 文档 | 内容 |
| --- | --- |
| [部署指南](docs/deployment.md) | 离线镜像、Compose、NAS / 1Panel、端口与目录、FUSE。 |
| [使用指南](docs/使用指南.md) | 账号、传输、STRM、有声书、整理、播放代理、设置。 |
| [升级与维护](docs/maintenance.md) | 持久化、备份、镜像与更新包升级、回退、常见故障。 |
| [插件使用](docs/plugins.md) | 来源、安装、权限、浏览器依赖、插件数据与排障。 |
| [外部 115 API](docs/外部115存储API标准.md) / [SDK](docs/外部115存储SDK.md) | 115 对外调用与 Go / Python 客户端。 |
| [外部 139 API](docs/外部139存储API.md) | 139 账号独立 Token、分页、上传与请求回执。 |

`/config` 保存设置、数据库、登录状态、插件数据与下载的运行组件，升级时必须沿用原目录。主程序镜像、主程序更新包、插件 ZIP、SDK ZIP 的用途不同，选择方法见 [升级与维护](docs/maintenance.md)。
