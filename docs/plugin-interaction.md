# 插件交互合同

插件通过项目已配置的 Telegram、企业微信接收命令、当前会话消息、按钮回调和附件，并在原会话回复。宿主管理机器人凭据、发送人白名单、路由、确认、持久化任务和发送回执。插件只实现声明动作，不维护另一套机器人轮询或企业微信回调服务。

合同保持 ABI v1、definition schema 2。没有 definition.interaction 的旧插件及现有状态、同步、回收站、链接转存、资源搜索入口沿用原流程。

## 声明与入口

清单顶层和 definition.permissions 同步声明权限。在 definition 中添加：

```json
{
  "permissions": [
    "interaction.receive", "interaction.read", "interaction.reply",
    "interaction.session", "interaction.attachments",
    "artifacts.read", "artifacts.write"
  ],
  "actions": [
    {"id":"start", "label":"开始交互", "kind":"run"},
    {"id":"input", "label":"接收输入", "kind":"run"},
    {"id":"finish", "label":"结束交互", "kind":"run"}
  ],
  "interaction": {
    "commands": [{"name":"work", "description":"打开插件工具", "action":"start"}],
    "message_action":"start",
    "attachment_action":"input"
  }
}
```

每插件最多 32 条命令，名称为 1–32 位小写英文字母、数字和下划线，首位为字母。描述为 1–100 字。交互目标动作必须为 run；宿主原生命令和 plugin、plugins、reply、cancel 等控制命令保留。接收插件必须声明 interaction.receive 和 interaction.read。安装启用后自动发现，停用、更新和卸载后旧上下文失效；SDK 调用还会复核当前授权。

| 用户输入 | 路由 |
| --- | --- |
| `/plugins` | 列出已启用且获准接收的插件入口 |
| `/plugin 插件ID work 参数` | 精确进入指定插件的命令，参数在 context.arguments |
| `/work 参数` | 只有一个已启用插件声明 work 时直接进入；重名时提示使用完整入口，不执行动作 |
| `/plugin 插件ID` | 执行 message_action；同条消息带附件且声明 attachment_action 时执行后者 |
| 后续普通文字、附件或分享链接 | 插件已调用 session.set 时投递下一步动作；没有等待会话时沿用原生消息流程 |
| `/reply 回调令牌` | 按钮的通用文本入口；令牌绑定原用户和会话 |
| `/cancel`、`取消` | 清除当前插件等待和按钮，并使旧任务的交互调用失效 |

Telegram 的 /plugins 加入现有机器人命令菜单。企业微信保留现有菜单，支持输入以上命令。普通影视名称不会因安装插件就被全局拦截；状态、全量/增量同步和原有回收站控制优先走原入口。插件需要自由文本时，先通过命令进入，再调用 session.set。附件没有插件归属且没有说明文字时，宿主提示先选择入口。

## 上下文与工具

Request.interaction 由宿主注入，普通网页动作、计划和存储驱动调用没有这个上下文。在 Input 中填写 interaction、用户 ID 或聊天 ID 不产生交互授权。WASM 只收到会话和消息标识、通道、文本、命令参数、插件状态、按钮数据及附件元数据；机器人 Token、企业微信密钥、供应商附件 ID 和下载 URL 留在宿主。

后台任务可把 ctx.delivery 保存到本插件 KV，稍后在交互工具输入中显式传 delivery，或调用 `InteractionContext(delivery)` 重新读取绑定。`InteractionReply`、`InteractionSession`、`InteractionSendFile` 有可选 Delivery 字段；EndInteraction 和 StageInteractionAttachment 也接受末尾的 delivery 参数。凭据只对原插件、当前步骤和有效会话生效，最长 24 小时，后续用户输入、更换插件步骤、取消或停用后拒绝使用。后台任务不接收任意聊天 ID，也不自动继承交互上下文；host.capabilities 的 interaction_required 表示需要当前或已保存的绑定。

| 工具 | 权限 | 输入与结果 |
| --- | --- | --- |
| interaction.context | interaction.read | 返回 InteractionContext：kind、text、command、arguments、state、callback、attachments、native_buttons、max_attachment_bytes |
| interaction.reply | interaction.reply | `{key,text,buttons?,ttl_seconds?}`；只回复当前会话，按钮为 `{id,label,action,data?}` |
| interaction.session.set | interaction.session | `{action,state?,ttl_seconds?}`；保存下一步动作和 JSON 状态 |
| interaction.session.end | interaction.session | 清除当前等待和按钮；当前动作仍可发送最后一条回复 |
| interaction.attachment | interaction.attachments | `{key,id}`；id 来自当前 attachments，返回 Artifact；导入还使用 artifacts.write，消费使用 artifacts.read |
| interaction.send_file | interaction.reply | `{key,input,caption?}`；input 为 `{artifact}`、`{media_task}`、`{file_task}` 之一，回发本插件输出 |

kind 为 command、message 或 callback。callback 含按钮 id 和 data。state 为 session.set 保存的 JSON，最多 16 KiB。按钮可指向其他已声明 run 动作。默认等待/按钮寿命 600 秒，最大 1800 秒；回复最多 16 KiB，最多 8 个按钮，按钮数据最多 4 KiB，单条输入最多 10 个附件。host.capabilities 同时公布这些上限；非交互任务的交互工具报告 interaction_required。

Telegram 使用原生 inline keyboard，并处理 answerCallbackQuery；企业微信将按钮显示为“标签：/reply 令牌”，两端执行相同的校验和回调流程。回调一次性消费，按通道、机器人配置、用户、聊天、话题和插件版本绑定，其他用户点击以及旧步骤的按钮均被拒绝。群聊中的不同用户分别持有自己的插件会话。

## SDK 使用

```go
ctx, err := pluginwasm.InteractionContext()
if err != nil { return pluginapi.Result{}, err }
if err = pluginwasm.WaitInteraction(pluginapi.InteractionSession{
    Action: "input", State: json.RawMessage(`{"step":"choose"}`), TTLSeconds: 600,
}); err != nil { return pluginapi.Result{}, err }
err = pluginwasm.ReplyInteraction(pluginapi.InteractionReply{
    Key: "choose", Text: "请选择，或发送文件",
    Buttons: []pluginapi.InteractionButton{{ID:"finish", Label:"结束", Action:"finish"}},
})
```

后续 input 动作读取 ctx.Text、ctx.State 和 ctx.Attachments。附件由宿主下载，不占用 WASM 的文件字节内存：

```go
artifact, err := pluginwasm.StageInteractionAttachment("input", ctx.Attachments[0].ID)
if err != nil { return pluginapi.Result{}, err }

// 需声明 media.read；宿主需要 FFprobe。
var probe map[string]any
err = pluginwasm.ProbeArtifactMedia(artifact.ID, &probe)

// 需声明 media.process；后续按 task.ID 查询媒体任务。
task, err := pluginwasm.SubmitMedia(pluginapi.MediaProcess{
    Key: ctx.Delivery, Artifact: artifact.ID,
    Operation: "transcode", Format: "wav", SampleRate: 16000, AudioChannels: 1,
})

// 将本插件生成的字幕、清单等产物回发会话。
err = pluginwasm.SendInteractionFile(pluginapi.InteractionSendFile{
    Key: "result", Input: pluginapi.HostInput{Artifact: artifact.ID},
})
```

media.probe 和 media.submit 的 path/file_task/artifact 三选一。Artifact 按内容探测；宿主保持输入占用直到媒体任务结束，期间产物释放被拒绝。产物仍可进入 service.upload、files.publish、shares.create 等链路；各自的权限、目录范围和确认要求继续生效。

```powershell
python tools/plugin-kit.py --out ../interaction-kit --id my-interaction --name 我的交互工具 --interaction
```

骨架只演示输入、等待、按钮和附件导入，业务识别、模型和文件处理由之后的插件实现。

## 确认、回执与生命周期

危险动作在提交插件队列前显示宿主确认，包含插件名、动作名和原 confirmation 文案。用户消费一次性确认令牌后，宿主才设置 Request.confirmed。插件按钮上的“确认”文字或回调 data 不产生确认授权；跳转到另一个危险动作时重新确认。

会话和按钮存储于宿主 plugin_interactions.db，正常重启后有效等待继续保留。新步骤替换旧投递，旧动作后续发送被拒绝。每次投递、回复和附件使用均检查机器人配置、白名单、当前安装、启用状态和工具授权。更换机器人、停用、更新、卸载、取消或新步骤阻止旧上下文继续发送。

入站消息按平台标识去重，回调一次性消费；动作进入原持久化任务队列。回复、附件导入和文件回发必须有 key，同一投递内相同 key 和参数复用回执，换参数被拒绝。回执保留 24 小时；发送前先持久化预留，发送结果未知时保留 attention，不用相同 key 自动重发。重启前中断的插件任务沿用原队列状态和重试规则，宿主不承诺对平台消息的事务性收取。

私有附件/输出回发上限为 20 MiB，实际还受平台 API 限制。Telegram 使用 getFile 和 sendDocument，参考 [Telegram Bot API](https://core.telegram.org/bots/api#getfile)；企业微信使用临时素材接口和应用消息，参考 [企业微信临时素材接口](https://developer.work.weixin.qq.com/document/path/90254)。大文件可沿用已授权的云盘暂存和限时输出分享。
