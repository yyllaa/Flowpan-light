# OpenList 网盘插件 0.1.0

安装 ZIP 并启用后，在账号中心选择 OpenList，填写服务地址、认证方式和根目录。多个 OpenList 实例、多个用户和同名 Flowpan 账号各自保存凭据与引用。根目录相对于 OpenList 用户的 base_path；服务地址保留反代路径前缀。

支持：账号密码（含登录时两步验证码）或 Token；分页浏览、文件详情、目录密码；通过 OpenList 代理的 Range 读取和播放；原生 STRM、有声书及 FUSE；建目录、改名、移动、复制、删除；流式上传及进度。权限以 OpenList 账号实际权限为准。单次批量改动最多 8 项，目录浏览每页最多 500 项，按上游目录排序显示；暂未接入搜索索引、分享转存和外部事件流。

要求包含 `storage.account-endpoint: 1` 和 `storage.path-refs: 1` 的 Flowpan 宿主。旧版宿主会在安装检查时提示更新。账号密码登录在服务明确返回 401 时尝试一次重新登录，校验身份后继续；Token 过期需更新 Token。两步验证会限制无人值守续期。

播放使用 OpenList `/p` 代理，流量经过 OpenList；不把账号 Token、密码放到播放 URL。服务须支持 `/p?force=true`，本版对照 OpenList v4.2.9 实测。外部存储驱动各自的限速、可写与复制能力由 OpenList 决定。

文件引用会跟随本插件发起的改名、移动和目录迁移并保存在 SQLite。通过其他客户端直接改名的文件需重新扫描。异步复制、移动按任务 ID 核验；未知写回执继续核验，避免重复操作。上传完整响应成功且大小核对后报告完成；恢复缺失回执时仅在服务提供匹配 SHA256 时自动确认，否则保留检查点等待核验，不重新上传。

构建：

```powershell
go test ./...  # 在 plugins/openlist 中运行
python build.py --out ../../storage-openlist-0.1.0.zip --go go
```

正式包已在固定宿主镜像及官方 OpenList 4.2.9 Local 驱动上完成真实 WASM 验证。源码单元测试覆盖续期、权限、限流、异步终态及未知上传回执；其他 OpenList 存储驱动和操作系统 FUSE 挂载需按实际环境验证。完整接入合同见 [网盘插件接入](../../docs/网盘插件接入.md)。
