# 插件通用接入合同

新增插件通过清单和 SDK 接入共用宿主。插件维护供应商协议，宿主负责新版提示、发布说明、仪表盘、账号、权限和 FUSE；新增品牌不增加共享页面分支，也不自行上报调用次数。

机器人入口使用可选 `definition.interaction` 及六项 `interaction.*` 工具，复用现有 Telegram、企业微信配置。命令、会话、按钮、附件、宿主确认和生命周期合同见 [插件交互](plugin-interaction.md)，独立开发包使用 `--interaction` 导出。

| 声明或调用 | 宿主自动行为 | 固定验证 |
| --- | --- | --- |
| `id`、数字 `version`、已启用的原插件源 | 刷新源并比较版本，显示更新数量与入口 | 两个新插件、不同版本格式、源隔离、失败重试 |
| `release.notes` | 应用内显示随版本保存的说明，本地 ZIP 无需插件源 | 本地无源、不同发布站点、纯文本 |
| `definition.storage.id/name` | 注册网盘，仪表盘使用真实标识和名称，首次请求前显示零 | 不同提供者、普通动作插件、无云盘探测 |
| `http.fetch`、`sdk.call`、`service.call` | 实际发送请求时由宿主计数 | 请求、拒绝、重定向、账号隔离及凭据脱敏 |
| `storage.capabilities` | 原生功能按已声明能力启用；可写 FUSE 检查完整写入能力 | 缺失能力在供应商调用前拒绝 |
| `pluginapi.StorageError` | SDK 保留业务码和 HTTP 状态，FUSE 使用实际提供者标签 | SDK 序列化、宿主恢复、未知提供者、旧适配器 |

## 发布说明

清单和插件源中的同一版本携带相同的 `release` 内容，安装包摘要继续绑定这些元数据：

```json
{
  "id": "my-storage-plugin",
  "version": "1.2.0",
  "release": {
    "title": "我的网盘 1.2.0",
    "notes": "新增目录移动。\n修复账号失效后的错误提示。",
    "published_at": "2026-11-14T23:00:00+08:00",
    "url": "https://example.com/releases/1.2.0"
  }
}
```

这是清单字段片段；完整清单仍包含运行时、权限和 definition。`notes` 最多 64 KiB，按纯文本显示，不执行 Markdown 内的 HTML；`title` 最多 256 字节，`published_at` 使用 RFC3339，`url` 使用 HTTPS。标题、时间和链接可省略，正文足以支持本地离线阅读。宿主不抓取任意发布页；声明的链接只在用户打开时跳转。

未声明 `release` 的旧包继续可安装；旧 GitHub 下载地址继续定位对应 Release。新版检查按原安装源匹配，源失效时显示错误，多个来源不会自动替换。相同版本和较低版本不提示更新；本地 ZIP 安装继续下载、导入流程。

新开发包默认生成 `release`、`CHANGELOG.md` 和发布检查：

```sh
python tools/plugin-kit.py --out ../my-plugin --id my-plugin --name 我的插件 --version 1.0.0
python tools/plugin-kit.py --out ../my-drive --storage --id my-drive --provider-id mydrive --name 我的网盘
flowpan plugin pack --release --dir ../my-plugin --out my-plugin.zip
flowpan plugin check --release my-plugin.zip
```

发布检查要求正文非空及可比较的数字版本，支持 `v1.2.0`、`1.2.0-rc.1`、`2026.11.14`。普通 `pack`/`check` 保留旧包兼容。升级时同步清单两层 `version`、`release.notes`、`CHANGELOG.md` 和插件源索引。新元数据与发布检查需要支持本合同的宿主。

## 仪表盘和 FUSE

仪表盘先从本地安装目录读取身份，即使零请求也显示插件；更新后的名称在下一次读取时生效。网盘使用 `storage.id` 分组，账号使用 `storage.id + ':' + Account`；普通动作使用 `plugin:{插件ID}`。身份从已安装清单和宿主创建的上下文读取，不接受供应商参数覆盖。

三种网络工具统计宿主实际发出的 HTTP 请求，允许的重定向按实际请求逐跳计数，本地 SDK 身份查询和校验拒绝不计数。统计不包含浏览器页面里的子请求或宿主传输的 CDN 内容；直链缓存命中不会增加元数据 API 计数。计数覆盖当前主程序进程，重启归零。仪表盘读取不执行插件，也不触发账号初始化、凭据校验或文件扫描。

网盘处理函数可返回通用错误，供应商 HTTP 状态和业务码不会丢失：

```go
return pluginapi.Result{}, &pluginapi.StorageError{
    Code: "ACCOUNT_EXPIRED", Status: 401, Message: "请重新登录账号",
}
```

SDK 的 `Serve(Handlers{Storage: ...})` 自动将错误放入 `storage_error`，宿主恢复该类型。业务码限 64 字节，仅允许字母、数字、点、下划线和连字符；状态为 0 或 100–599。旧插件返回普通错误仍保持原行为。

共享 FUSE 读取、同步和上传提示取实际提供者；未知来源显示 `source`，内置适配器保留已有标签。成功日志不输出零状态、空错误码和 `<nil>`。错误日志只读取 `HTTPStatus()` 与 `BusinessCode()`，不输出错误正文、文件名、签名地址或凭据。缓存及单飞语义保持一致。

## 项目回归入口

```sh
npm run check:plugin-contract
npm run check:frontend
```

合同检查要求 Go 1.25 及 Node 22，运行后端、SDK、发布门禁和前端行为测试；现有全项目检查同样包含这些测试。CI 可用 `FLOWPAN_GO` 指定 Go 命令，`FLOWPAN_GO_ARGS` 提供 JSON 参数数组以接入已有容器运行环境。

真实浏览器用例 `tests/browser/plugin-adaptive.js` 已加入项目浏览器检查，覆盖 1440、390、320 像素，两种全新插件身份、非 GitHub 说明、无 URL 说明、本地无源说明、零自动安装和 HTML 纯文本。验收时结合原更新用例检查旧 GitHub 插件。

后续开发可直接采用以下要求：

```text
按 docs/plugin-integration-contract.md 接入新插件，优先使用 tools/plugin-kit.py。
供应商协议放在插件中；共享宿主、页面、仪表盘和日志不增加品牌或插件 ID 分支。
随版本填写 release.notes，数字版本可比较，使用 pack/check --release。
网盘能力按 storage.capabilities 声明，错误使用 pluginapi.StorageError。
运行 check:plugin-contract、完整前端检查及对应桌面和手机浏览器用例。
同时验证另一提供者及普通动作插件，保留旧包兼容与来源、账号隔离。
```


## 云盘文件处理

音乐、字幕等插件通过通用 `files.stage`、媒体 `file_task` 输入和 `files.publish` 衔接网盘文件。账号与目录沿用当前授权，上传与恢复使用明确冲突策略，未知回执不自动重放；能力按每个驱动实际支持情况返回。SDK 开发包使用 `--files` 导出示例，字段与恢复流程见 [云盘文件处理合同](plugin-file-pipeline.md)。
