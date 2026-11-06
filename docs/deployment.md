# Docker 与 NAS 部署指南

本文使用已经构建好的 Flowpan 主程序镜像。仓库提供可直接使用的 [Compose 配置](../deploy/compose.yaml)，无需下载或构建主程序源码。

## 准备文件与运行环境

- Docker Engine 与 Docker Compose 插件，NAS 可使用兼容的容器管理界面。
- 与主机 CPU 架构匹配的 Flowpan 主程序镜像包。`linux/amd64` 与 `linux/arm64` 应按实际交付的平台选择。
- 可持久保存配置的部署目录，下载运行组件和处理本地媒体时还需预留磁盘空间。

目前仓库 [Releases](https://github.com/yyllaa/Flowpan-light/releases) 提供插件和 SDK，主程序镜像按单独交付的离线文件导入。GitHub 自动生成的 `Source code.zip`、`resource-dian-*.zip` 和 `Flowpan-Plugin-SDK.zip` 均不是主程序镜像包。

## 通过 Compose 部署

以下命令在 Linux / NAS 终端执行。部署目录结构为：

```text
flowpan/
  .env
  deploy/
    compose.yaml
    compose.fuse.yaml    # 仅启用 FUSE 时需要
  config/
  logs/
  media/
```

### 1. 下载部署配置

```bash
mkdir -p flowpan/deploy
cd flowpan
curl -fL https://raw.githubusercontent.com/yyllaa/Flowpan-light/main/deploy/compose.yaml -o deploy/compose.yaml
curl -fL https://raw.githubusercontent.com/yyllaa/Flowpan-light/main/deploy/.env.example -o .env
mkdir -p config logs media
```

也可以在 GitHub 分别打开这些文件并下载到相同位置。`.env` 位于部署目录根部，Compose 文件位于 `deploy/` 中。

### 2. 导入镜像并填写标签

将离线包上传到服务器，使用实际路径导入：

```bash
docker load -i /path/to/flowpan-image.tar.gz
docker image ls
```

在 `.env` 中填写导入结果 `Loaded image:` 后的完整标签：

```dotenv
FLOWPAN_IMAGE=flowpan-light-core:实际导入的版本标签
TZ=Asia/Shanghai
FLOWPAN_WEB_PORT=8080
FLOWPAN_EMBY_PORT=8081
FLOWPAN_FNTV_PORT=8082
```

`实际导入的版本标签` 是需要替换的说明文本，不能原样作为镜像名称。`.env.example` 中的 `flowpan-light-core:installed` 同样是本地标签占位值。本示例不指定未经发布的 Docker Hub / GHCR 地址，也不会尝试拉取占位镜像。

### 3. 检查并启动

```bash
docker compose --env-file .env -f deploy/compose.yaml config --quiet
docker compose --env-file .env -f deploy/compose.yaml up -d --pull never
docker compose --env-file .env -f deploy/compose.yaml ps
docker compose --env-file .env -f deploy/compose.yaml logs --tail=100
```

浏览器访问 `http://服务器IP:8080`。首次访问创建管理员账号，再添加网盘账号。修改了 `FLOWPAN_WEB_PORT` 时，访问地址使用修改后的宿主机端口。

本配置的服务名是 `flowpan-core`。后续在同一部署目录使用相同的 Compose 参数，项目名固定为 `flowpan`。

## 端口与目录

| 容器端口 | 默认宿主机端口 | 用途 |
| --- | --- | --- |
| 8080 | 8080 | 管理后台、API、STRM 跳转与回调。 |
| 8081 | 8081 | Emby 代理，需在页面配置并启用。 |
| 8082 | 8082 | 飞牛影视代理，需在页面配置并启用。 |

端口左侧可在 `.env` 中修改；右侧对应服务容器内的监听端口。功能页修改了监听端口时，也要修改映射的容器端口。未使用的代理端口可从 Compose 的 `ports` 中移除。

| 宿主机目录 | 容器目录 | 内容 |
| --- | --- | --- |
| `flowpan/config` | `/config` | 设置、SQLite 数据库、登录状态、插件私有数据、运行组件与更新状态。 |
| `flowpan/logs` | `/logs` | 项目运行日志。 |
| `flowpan/media` | `/media` | 本地媒体、STRM 输出及页面设置引用的目录。 |
| 可选共享目录 | `/cloud` | 启用 FUSE 后的统一挂载父目录。 |

相对绑定路径按第一个 Compose 文件所在目录解析。本配置写的是 `../config`、`../logs`、`../media`，因此它们落在 `flowpan/` 下。若把 Compose 文件移动到其他位置，应同步改成正确的绝对路径。

需要使用现有媒体目录时，可将 `../media:/media` 改为宿主机实际媒体目录的绝对路径。页面填写容器内路径，例如 `/media/strm`；其他容器使用这份文件时映射同一个宿主机目录。

## NAS / 1Panel 图形界面

使用界面导入镜像后，创建容器并选择实际导入的标签；也可以在 Compose / 编排功能中使用本文示例，填写对应环境变量与绝对目录。

| 项目 | 设置 |
| --- | --- |
| 镜像 | 已导入的 Flowpan 主程序标签。 |
| 命令 | 沿用镜像默认启动命令。 |
| 环境变量 | `FLOWPAN_ROOT=/app`、`FLOWPAN_HOME=/config`、`FLOWPAN_LOGS_DIR=/logs`、`FLOWPAN_MEDIA_DIR=/media`、`TZ=Asia/Shanghai`。 |
| 目录映射 | 持久化 `/config`、`/logs`、`/media`。 |
| 端口 | 按需映射 8080 / 8081 / 8082。 |
| 重启策略 | `unless-stopped` 或界面对应的自动重启选项。 |

普通文件管理、STRM、转存和插件不要求 FUSE 设备权限。使用挂载功能时再追加下一节配置。

<a id="fuse"></a>

## 可选：FUSE 云盘挂载

这部分适用于提供 `/dev/fuse` 和共享挂载传播能力的 Linux / NAS。Windows / macOS Docker Desktop 的宿主机目录映射与 Linux 挂载传播不同，应按实际平台选择运行方式。

多个账号只映射一个 `/cloud` 父目录，选择的来源目录直接显示在其下：

```text
/cloud/有声书      # 例如 115 中选定的 /有声书
/cloud/音乐        # 例如 139 中选定的 /音乐
```

不需要按网盘或账号再建立一层父目录。来源重名时以页面显示的区分名称为准。

### 1. 准备宿主机共享父目录

先检查设备：

```bash
ls -l /dev/fuse
```

选择一个专门用于云盘挂载的目录。下面以 `/srv/flowpan/cloud` 为例，使用管理员权限在宿主机执行，路径按实际环境修改：

```bash
mkdir -p /srv/flowpan/cloud
mountpoint -q /srv/flowpan/cloud || mount --bind /srv/flowpan/cloud /srv/flowpan/cloud
mount --make-rshared /srv/flowpan/cloud
findmnt -o TARGET,PROPAGATION /srv/flowpan/cloud
```

共享挂载传播需要在宿主机重启后仍然建立，可通过 NAS 的启动任务或发行版挂载配置维护。应在启动 Flowpan 和使用挂载的其他容器之前完成。

### 2. 启用可选配置

```bash
curl -fL https://raw.githubusercontent.com/yyllaa/Flowpan-light/main/deploy/compose.fuse.yaml -o deploy/compose.fuse.yaml
```

在 `.env` 追加实际宿主机目录：

```dotenv
FLOWPAN_CLOUD_DIR=/srv/flowpan/cloud
```

检查并重新创建服务：

```bash
docker compose --env-file .env -f deploy/compose.yaml -f deploy/compose.fuse.yaml config --quiet
docker compose --env-file .env -f deploy/compose.yaml -f deploy/compose.fuse.yaml up -d --pull never
```

可选配置增加 `/dev/fuse`、`SYS_ADMIN`、AppArmor 配置和 `/cloud` 的 `rshared` 绑定。然后进入项目挂载页面，选择账号与来源目录并启用挂载。

启用后，后续重建和升级继续带上这两个 `-f` 参数。

### 3. 提供给其他容器

播放器等容器使用同一个宿主机父目录，并接收子挂载传播，例如：

```yaml
volumes:
  - /srv/flowpan/cloud:/cloud:ro,rslave
```

只读播放器可使用 `ro,rslave`。权限还要与挂载页面中允许访问的用户设置一致。在 Flowpan、宿主机及播放器容器内分别检查选定目录，确认三个位置都能列出内容。

## 部署后检查

完成管理员初始化和账号登录后，按需下载 FFmpeg / 浏览器，检查 STRM、文件浏览、插件和播放入口。配置顺序见 [使用指南](使用指南.md)，备份、升级和故障排查见 [升级与维护](maintenance.md)。
