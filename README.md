# Flowpan-light

Flowpan-light 是一个面向 115 网盘的本地服务，包含文件管理、分享转存、STRM 同步、媒体整理与刮削、Emby 播放代理、通知互动和辅助插件。

本仓库同时发布 Flowpan 外部 115 存储 SDK。完整部署与设置请阅读 [使用指南](./docs/使用指南.md)，外部应用接入请阅读 [外部 115 存储 API 标准](./docs/外部115存储API标准.md) 和 [SDK 文档](./docs/外部115存储SDK.md)。

## 快速开始

准备好 Flowpan 服务源码与 Docker 环境后，在服务源码根目录执行：

```bash
docker compose up -d --build
docker compose logs -f --tail=200
```

浏览器访问 `http://<服务器IP>:8080`，首次创建 Flowpan 管理员账号，再按顺序完成：

1. **账号**：扫码登录 115 主账号。
2. **其他设置**：按需配置 HTTP 代理、OpenAPI、通知、AI 和外部 API。
3. **STRM 配置**：选择本地输出目录和 115 源目录，保存后执行首次同步。
4. **Emby 代理**：填写 Emby 地址与 API Key，选择 `302` 或本地代理模式。

默认 Docker Compose 使用：

```yaml
services:
  flowpan:
    ports:
      - "8080:8080" # 管理后台、API、STRM 跳转
      - "8081:8081" # Emby 本地代理
    volumes:
      - ./config:/config # 设置、Cookie、SQLite、更新状态
      - ./logs:/logs     # 运行日志
      - ./media:/media   # 本地媒体、STRM 或有声书目录
```

`config/` 保存管理员账号、115 登录状态和所有 Web 设置。不要删除它；升级镜像前先备份整个目录。

## 功能边界

- 删除、移动、转存和回收站清理会操作 115 云端文件，执行前确认当前账号和目录。
- Emby 功能提供播放代理、Webhook 通知、媒体库刷新和用户播放策略；不提供“Emby 删除媒体后删除 115 文件”的联动能力。
- 外部应用应使用独立外部 API Token 调用 `/api/v1/storage/115/...`，不能复用管理员密码、115 Cookie 或通知密钥。

## 外部 115 存储 SDK

SDK 调用 Flowpan 的标准接口：

```text
/api/v1/storage/115/{driver}/{action}
```

它允许其他后端项目使用 Flowpan 暴露的 115 存储能力，而不会拿到 Flowpan 管理员账号、115 Cookie 或 OpenAPI 凭据。

支持的 driver：

- `auto`：推荐。OpenAPI 已授权时使用 OpenAPI，否则使用 Cookie。
- `open`：强制 OpenAPI。
- `cookie`：强制 Cookie。

### 安装

Go：

```bash
go get github.com/yyllaa/Flowpan-light@v0.2.0
```

Python：

```bash
pip install .
```

### Go 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/yyllaa/Flowpan-light/sdk/go/flowpan115"
)

func main() {
    client := flowpan115.New("http://127.0.0.1:8080", "external-token")
    ctx := context.Background()

    client.SetDefaultDriver(flowpan115.DriverAuto)
    usage, err := client.Usage(ctx, "")
    if err != nil {
        panic(err)
    }
    fmt.Println(usage.Driver, usage.Result.Used, usage.Result.Available)
}
```

### Python 示例

```python
from sdk.python.flowpan115 import DRIVER_AUTO, Flowpan115Client

client = Flowpan115Client("http://127.0.0.1:8080", "external-token")
client.set_default_driver(DRIVER_AUTO)
usage = client.usage()
print(usage.driver, usage.result["used"], usage.result["available"])
```

### 鉴权与权限

在 Flowpan 的 **其他设置** 启用外部 API，生成独立 Token 后使用：

```http
Authorization: Bearer <external_api_token>
```

按调用方需要授予最小权限：

- `read`：列表、搜索、文件详情、用量、下载链接。
- `write`：新建目录、移动、复制、粘贴、重命名、星标。
- `upload`：上传桥接操作。
- `delete`：删除。

不要复用 MoviePilot 事件密钥、Flowpan 管理员密码或 115 Cookie。

### SDK 可靠性约定

SDK 只会对读取类动作和上传状态/签名查询重试短暂故障；不会盲目重试移动、复制、重命名、删除或上传完成，因为网络超时后写入操作可能已经在服务端成功。

两个 SDK 都会拒绝非法 driver 和格式错误的 action 路径。业务必须固定使用某条 115 链路时，应显式选择 `open` 或 `cookie`。上传初始化的 typed helper 会返回完整会话信息，调用方应原样保存 `session_id`、`upload_id`、回调数据与分片元数据。

## 文档

- [完整部署与使用指南](./docs/使用指南.md)
- [外部 115 存储 API 标准](./docs/外部115存储API标准.md)
- [外部 115 存储 SDK](./docs/外部115存储SDK.md)

## 外部插件与开发 SDK

插件源由用户手动添加：`https://github.com/yyllaa/Flowpan-light/tree/main/plugins`。索引位于 [`plugins/index.json`](plugins/index.json)，当前提供癫影外部 WASM 插件；本仓库的插件源不列入宿主内置功能。

- [独立插件 SDK 源码](sdk/plugins) · [插件开发与接入](docs/plugin-development.md) · [24 项宿主工具](docs/plugin-host-tools.md)
- [癫影插件源码与使用说明](plugins/dian)
- [下载插件与 SDK](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06)

浏览器、代理、FFmpeg、转存、整理和通知能力由 Flowpan 宿主提供；浏览器在项目“其他设置”中按需下载维护。插件安装、账号配置与授权在“辅助插件 → 插件管理”完成。

构建需要 Go 1.25+ 和 Python 3：

```sh
python plugins/build.py --out dist/plugins
```

生成可直接导入的癫影 ZIP、独立 SDK ZIP 和 SHA-256。开发插件可运行 `python tools/plugin-kit.py --out ../my-plugin --id my-plugin --name 我的插件` 导出独立工程。
