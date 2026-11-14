# 云盘文件处理合同

云盘音乐标签、字幕和其他媒体处理插件通过宿主文件任务搬运数据，通过媒体任务调用共享依赖。大文件不进入 WASM 内存，不使用 `http.fetch` 承载音频，也不向插件返回账号凭据或签名下载链接。

## 接入流程

1. 声明 `host.read`、`files.read`、`files.publish`、`media.read`、`media.process`，按需声明元数据站点的 `http.fetch` 和 `http_hosts`。发布或恢复动作声明 `danger: true` 和确认文案。原有账号、目录授权继续生效。
2. 调用 `host.capabilities` 检查宿主合同，`files.capabilities({account})` 检查该账号支持的策略，`media.status` 检查 FFmpeg/FFprobe。依赖继续在项目设置的“依赖下载”入口按需安装。
3. `files.stage({key, account, id})` 返回异步暂存任务。宿主直接从网盘读取到该插件的私有任务区，核对文件大小、供应商提供的可识别哈希，并记录 SHA-1、SHA-256、MD5；下载前后核对原始快照。
4. 通过 `files.task({id})` 查询至 `completed`。使用 `media.probe({file_task})` 探测标签，使用 `media.submit({key, file_task, operation, format, metadata})` 提交处理。`path` 与 `file_task` 二选一。旧本机路径合同继续兼容。
5. 通过 `media.task` 查询至 `completed`。`files.publish` 直接读取本插件的媒体结果，支持同账号或另外一个已授权账号的输出目录。账号以真实 ID 绑定，同名账号不混用。
6. 回写完成后调用 `media.release` 和 `files.release` 清理本地结果。云盘回写、原始快照与备份引用的回执继续保留。正在排队或执行回写的媒体结果保持占用，释放操作会拒绝。

每个动作提交或查询一步，页面通过任务 ID 展示进度；长文件处理不应在同一次 WASM 调用里持续轮询。任务幂等键相同且参数相同会返回同一任务，参数、账号授权版本或插件包变化时拒绝复用。

## 回写参数与同名策略

```json
{
  "key": "publish-file-version-1",
  "account": "src_example",
  "parent_id": "music-directory-id",
  "name": "歌曲.flac",
  "media_task": "本插件已完成的媒体任务ID",
  "conflict": "replace",
  "original_task": "该媒体输入对应的暂存任务ID"
}
```

| 策略 | 条件与行为 |
| --- | --- |
| `keep_both` | 使用带任务标识的独立名称上传并校验，保留所有原文件。需 list/stat/read/upload。 |
| `skip` | 存在同名文件时直接跳过；不存在时先以独立临时名上传并校验，再检查名称并改为目标名。还需 rename。 |
| `replace` | 必须提供媒体输入对应的原始暂存任务；目标需同账号、同目录、同名称，且只有一个匹配原 ID 的文件。先上传校验新文件，再复核原文件快照与内容，将原文件改为备份名，最后提交新文件。还需 rename。 |

`files.capabilities` 返回 `stage`、`publish`、`conflicts`、`atomic_replace`、实际 `operations` 和上限。当前宿主的替换采用多个操作，`atomic_replace` 为 false，文件 ID 会变化；需要稳定 ID 或原子版本更新的使用方应选择另存，或等待对应的原生版本更新合同。宿主不按品牌推测同名上传行为；原生文件操作通过单独的结构化工具、权限及幂等回执开放，不提供任意路径上传或云盘凭据。

`files.publish` 的输入现在可在 `media_task`、`artifact`、`file_task` 中选择一个。`artifact` 用于插件生成的字幕、清单、STRM、密文等已封存字节，`file_task` 用于已完成暂存的直接中转。本机输出当前支持 `keep_both`，返回的 `task.result.id/name` 是实际路径和文件名，不能根据请求文件名猜测结果路径。新字段全部可选，原有媒体回写流程保持原样。

宿主检查自己任务之间的名称占用，并在改名之前重新检查。外部客户端仍能在两个操作间修改目录；没有供应商条件写入能力时，不能把多步操作宣称为原子提交。异常时保留阶段、已上传 ID、备份 ID，供核验和恢复。

## 校验、恢复与中断

- 上传完成回执必须包含正确来源、父目录、名称、大小和文件 ID。目标提供 SHA-1、SHA-256 或 MD5 时核对哈希；没有共同哈希时，宿主回读新文件计算 SHA-256。校验通过后才移动原文件。
- 原文件在任务期间发生 ID、父级、名称、大小、修改时间、版本或已知哈希变化时停止替换；提交前再次校验原始内容。恢复时允许宿主改名带来的时间/版本变化，仍核对原文件 ID、父级、大小和内容。
- `files.restore({id, key})` 指向原替换任务。已处理文件先改为保留名称，再把原文件备份恢复成原名，恢复原文件 ID。没有开始交换时可直接确认原文件仍在原位。恢复会保留处理结果，不删除原始内容。
- `attention` 表示云盘写入结果待核验，可能包括已经接受上传但响应丢失、改名响应丢失或提交被中断。同一键不自动重传或重做改名；恢复操作读取真实文件 ID 和当前名称决定下一步。
- 服务重启后未完成下载标记 `interrupted`，未完成回写标记 `attention`，保留写入前阶段和检查点。当前不自动续传，不自动重放写操作。取消、插件更新、回退和卸载等待正在执行的文件/媒体任务停止。
- 每个文件最大 20 GiB，单插件暂存预留最多 40 GiB；全局最多 8 个未结束文件任务、2 个传输工作位、1 个回写提交位，单任务最多 2 小时。媒体任务沿用原有进程和结果上限。使用方应在消费完成后显式释放结果。

## SDK 示例

```go
// 每个调用都应处理 error；将返回 ID 保存在自己的任务检查点中。
staged, err := pluginwasm.StageFile(pluginapi.FileStage{
    Key: "source-v1", Account: account, ID: objectID,
})
// 下一次动作查询 staged.ID 至 completed，再读取与匹配音乐信息。
processed, err := pluginwasm.SubmitMedia(pluginapi.MediaProcess{
    Key: "tags-v1", FileTask: staged.ID, Operation: "remux", Format: "flac",
    Metadata: pluginapi.MediaMetadata{Title: title, Artist: artist, Album: album},
})
// 下一次动作查询 processed.ID 至 completed；已确认的 run 动作中回写。
published, err := pluginwasm.PublishFile(pluginapi.FilePublish{
    Key: "write-v1", Account: account, ParentID: parentID, Name: name,
    MediaTask: processed.ID, OriginalTask: staged.ID, Conflict: "replace",
})
```

用 `python tools/plugin-kit.py --out ../music-workflow --id music-workflow --name 音乐处理 --files` 导出可独立构建的示例，包含能力检查、暂存、探测、标签处理、回写、进度、取消、恢复和释放动作。它接受已确定的音乐标签；音乐站点查询、低置信度选择等规则由业务插件实现。现有媒体合同支持固定文字标签、重新封装和封面提取；新增封面嵌入、歌词字段或其他原生依赖时在媒体合同中统一扩展。
