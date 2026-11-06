# 升级与维护

本文沿用 [部署指南](deployment.md) 的目录结构与服务名。命令在部署目录 `flowpan/` 下执行；启用 FUSE 的部署，在相关 Compose 命令中同时带上 `-f deploy/compose.fuse.yaml`。

## 文件包用途

| 文件 | 使用位置 |
| --- | --- |
| 主程序离线镜像 `.tar` / `.tar.gz` | 通过 `docker load -i` 导入，再由 Compose 创建容器。 |
| 主程序更新包 | 在“其他设置 → 系统更新”选择并安装，必须匹配当前环境及架构。 |
| `resource-dian-版本.zip` 等插件包 | 在“插件市场”本地导入，或通过插件来源安装。 |
| `Flowpan-Plugin-SDK.zip` | 外部插件开发使用。 |
| GitHub 自动生成的 `Source code.zip` | 仓库文件快照，不能作为主程序镜像或插件安装包。 |

当前仓库 [Releases](https://github.com/yyllaa/Flowpan-light/releases) 提供插件与 SDK，主程序离线镜像按单独交付的文件使用。对照文件用途选择安装入口。

## 持久化与备份

`/config` 是首要备份目录，包含数据库、登录状态、插件配置与数据、下载的 FFmpeg / 浏览器、更新状态等。升级镜像时沿用原来的目录映射。`/media` 中的本地文件和 STRM 输出按自身备份策略保留；`/logs` 用于排障。

SQLite 数据库在运行中会有写入。以下示例先停止服务，再备份整个配置目录和部署文件：

```bash
docker compose --env-file .env -f deploy/compose.yaml stop
mkdir -p backups
tar -czf "backups/flowpan-config-$(date +%Y%m%d-%H%M%S).tar.gz" config deploy .env
docker compose --env-file .env -f deploy/compose.yaml start
```

备份文件含登录状态和服务配置，应保存在自己的备份位置。备份目录不要放在本次打包的 `config/` 内。云盘源文件不包含在这个配置备份中。

## 替换主程序镜像

1. 记录当前 `.env` 中的镜像标签和后台版本，并完成配置备份。
2. 导入新镜像，确认 `docker load` 返回的新标签。
3. 修改 `.env` 中 `FLOWPAN_IMAGE` 为新标签。
4. 使用原目录映射重新创建容器。

```bash
docker load -i /path/to/new-flowpan-image.tar.gz
# 编辑 .env，填写上一步导入的新镜像标签。
docker compose --env-file .env -f deploy/compose.yaml config --quiet
docker compose --env-file .env -f deploy/compose.yaml up -d --force-recreate --pull never
docker compose --env-file .env -f deploy/compose.yaml ps
docker compose --env-file .env -f deploy/compose.yaml logs --tail=100
```

这是本地导入镜像的流程，无需执行镜像拉取。只有明确使用已发布的镜像仓库地址时，才按该镜像的发布方式更新。

重建后确认后台版本、网盘登录状态、原目录绑定和插件安装状态。浏览器和 FFmpeg 位于 `/config`，沿用该目录可继续使用；组件版本在各自设置中单独更新。

## 使用系统更新包

在“其他设置 → 系统更新”查看当前运行环境，选择匹配环境与 CPU 架构的主程序更新包，安装完成后重启容器：

```bash
docker compose --env-file .env -f deploy/compose.yaml restart flowpan-core
```

内置更新状态保存在 `/config/updates`，容器入口负责加载相应运行版本。升级后检查后台当前版本及启动日志。插件更新走插件管理页面，不能把插件包放到系统更新入口。

## 回退版本

保留上一版镜像标签和升级前配置备份。需要回退时停止当前容器，把 `.env` 改回上一版标签，再用原映射重新创建。若该版本存在数据库格式变化，应按对应版本说明使用升级前的配置备份，先另外保留当前配置副本。

同一套配置目录只交给一个实例运行。重新创建后检查显示版本、账号状态和目录绑定，再恢复定时任务及依赖的播放服务。

## 常用检查

```bash
docker compose --env-file .env -f deploy/compose.yaml ps
docker compose --env-file .env -f deploy/compose.yaml logs --tail=200 flowpan-core
docker compose --env-file .env -f deploy/compose.yaml exec flowpan-core flowpan version
docker compose --env-file .env -f deploy/compose.yaml exec flowpan-core df -h /config /media
```

后台日志、传输回执、插件运行记录及书库扫描状态分别对应不同任务。排查时先确定网盘账号、目录、任务时间和具体错误，再查看相关记录。

## 常见问题

| 表现 | 检查方法 |
| --- | --- |
| 提示镜像不存在 | 确认已成功执行 `docker load`，`.env` 标签与 `Loaded image:` 完全相同；占位标签需替换。 |
| 后台打不开 | 检查容器状态、启动日志、宿主机端口、防火墙及访问地址。客户端的 `127.0.0.1` 只代表当前设备。 |
| 重建后要求重新初始化 | 检查是否沿用了原 `/config`，以及相对绑定路径是否因移动 Compose 文件而变化。 |
| FFmpeg / 浏览器未就绪 | 在“其他设置”检查安装状态、下载源、代理、平台支持与磁盘空间；组件需单独下载。 |
| 插件包下载超时 | 保存容器可访问的项目代理，检查 GitHub 连接后重试；GitHub Token 处理认证或额度，不能代替网络连接。 |
| GitHub 来源能刷新但安装失败 | 索引与 Release 包使用不同下载地址，检查插件包错误及代理，并更新到包含相应下载修复的主程序版本。 |
| 音频探测失败 | 检查 FFprobe 就绪状态、文件可读性、音频格式和相应扫描日志；失败本身不代表正在上传。 |
| 书库等待目录稳定 | 查看该账号绑定目录的变更、实际传输和复核状态；未完成任务、探测错误分别排查。 |
| 转存 / 复制未完成 | 查看任务回执和目标目录；分享转存还需确认分享可访问及审核状态，115 跨账号秒传使用独立链路。 |
| STRM 有文件但不能播放 | 检查文件内地址是否可由播放器访问、对应账号状态及代理模式。 |
| 挂载仅容器内可见 | 检查宿主机父目录是否共享、Flowpan 是否使用 `rshared`、其他容器是否使用 `rslave`，以及目录权限。 |
| Emby / 飞牛影视无法访问 | 检查功能已启用、目标服务地址可由容器访问，以及监听端口与映射一致。 |

## 插件数据维护

插件卸载默认保留配置、私有数据和登录状态；需要一起清理时，在卸载弹窗勾选“同时移除插件数据”。重新安装、换来源与数据清理的区别见 [插件使用](plugins.md)。
