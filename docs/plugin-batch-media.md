# 长批次任务与媒体处理合同

以下能力从宿主 2026.11.17-music-runtime 起提供，适用于声明相应能力的插件。短任务、文件发布与识别整理流程继续使用原合同。

## 分段任务

动作声明 `kind: "run"`、`continuable: true` 后，可以在成功结果中返回 `continuation: {"delay_seconds": 3}`。延迟范围 1–86400 秒，最多 100000 段。宿主将同一任务重新排队，保留冻结的配置、输入、确认状态、授权范围、进度和任务 ID，增加 `steps` 并写入 `next_run_at`。正常结束才生成完成事件；取消后不再调度。

插件必须先持久化检查点，再返回 continuation。重启恢复待执行分段；运行中断仍遵循宿主原有待核验机制。副作用必须通过幂等工具与稳定 key 提交，不能把分段再次调用解释为再次执行上传。`host.capabilities.limits.continuation_seconds` 大于零时可用。

```go
if err := pluginwasm.Set("checkpoint", checkpoint); err != nil { return pluginapi.Result{}, err }
return pluginapi.Result{Message:"等待下一段", Continuation:&pluginapi.Continuation{DelaySeconds:3}}, nil
```

## 二进制图片与媒体

`http.fetch` 可指定 `response_encoding: "base64"`，返回 `encoding: "base64"`、原始响应头与编码后的 body，二进制最大 512 KiB。域名声明、代理、请求计数和宿主网络控制沿用原规则。可将图片分块写入本插件的 artifact，再封存。

`media.submit` 的 remux 请求可指定 `cover_artifact`：同一插件、同一有效安装和授权范围下已封存的 JPEG/PNG，最大 512 KiB、4096×4096。支持 mp3/flac/m4a/m4b。宿主持有图片租约，将受控图片送给 FFmpeg；插件不提供任意 FFmpeg 参数。`metadata.lyrics` 支持换行、最大 16 KiB，其他标签仍使用原长度规则。音频使用 copy；可用音频包 SHA-256 验证不发生重编码。

`host.capabilities.limits.media_artwork_bytes` 大于零时可用。媒体任务通过 `media.task` 跟踪，发布通过 `files.publish`，结束后释放 `media.release` 与 `files.release`。

## 文件列表与状态

网盘 `files.list` 每条记录和 `files.stat` 返回顶层 `id`，与 `pluginapi.StorageEntry` 对齐，同时保留旧 `ref` 身份字段。本地记录包含 parent_id、path、modified、version，列表与 stat 使用同一口径。

`kv.get` 只读取被请求键的索引记录，保持过期判断和安装隔离；不再为单个键加载整份状态。`kv.set` 的键仍限 64 个小写字母、数字和短横线。批次状态使用分片记录，每段仍受 128 次宿主调用、64 KiB 请求和 1 MiB 响应限制。
