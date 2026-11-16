# 插件识别与分类扩展合同

插件向原整理链路补充关键词、格式匹配、身份查询和分类声明。用户继续使用原来的转存入口、待整理目录和“立即整理”；启用插件后自动生效，分类目录在原来的“分类策略”中绑定。宿主负责实际文件、目录、命名及操作，插件独立维护查询来源和关键词。

## 插件声明

`python tools/plugin-kit.py --recognition --id my-catalog --out <新目录>` 导出独立 Go/WASI 工程、SDK 和本合同。骨架返回 `no_match`，开发者实现真实来源查询后再用于整理。

在 `definition.recognition` 中声明：

```json
{
  "categories": [{ "id": "special", "name": "小姐姐" }],
  "providers": [{
    "id": "catalog",
    "name": "专用识别",
    "extensions": [".mkv", ".mp4", ".strm"],
    "keywords": ["sample-key", "sample-label"],
    "patterns": ["(?i)\\bNX-[0-9]+\\b"],
    "category": "special"
  }]
}
```

扩展名先过滤；关键词或正则任一匹配即可认领。关键词按字面子串匹配，忽略大小写，不当作正则。这里的样例词没有业务含义，插件替换为自己维护的特征词。没有关键词和正则的处理器只供显式工具调用，不自动接管。每个插件最多 16 个处理器、16 个分类；每个处理器最多 64 个关键词。

分类 ID 使用插件内的稳定标识；宿主绑定为 `my-catalog/special`，同名显示分类不会相互覆盖。插件声明分类名称，用户选择实际目录。停用或卸载后保留已保存绑定；重新安装相同 ID 可以继续使用。

## 原整理链路

1. 资源通过原转存或交互入口进入已配置的待整理目录；需要等待真实文件入盘。
2. 用户点击原来的“立即整理”，或由原有自动触发启动。
3. 宿主检查启用插件的扩展名、关键词和正则。没有插件认领时继续原有 AI/TMDB 流程。
4. 命中的插件通过 `Recognition` 生命周期查询来源。唯一确认身份后，宿主使用原命名逻辑和对应分类目录执行整理。
5. 已认领却未确认、查询失败、目录未绑定或多插件确认冲突时，文件保留待确认。此时不会进入 TMDB。

启用插件是自动接入的开关。原生任务不再读取旧的 `recognition.mode/providers/rules/destination` 来决定分流，网页也不再提供单独的“识别来源”页。处理器的声明不包含网盘目录 ID；目录绑定仍按当前账号隔离。普通影片、剧集的分类规则及全局命名保持原逻辑。

“分类策略”自动显示插件分类，绑定后写入原有 `category_rules`：

```yaml
movie:
  原电影分类: {}
tv:
  原剧集分类: {}
plugin:
  my-catalog/special:
    cid115: "实际目录ID"
    path115: "/小姐姐"
```

115 使用 `cid115/path115`；139 与通用存储账号沿用现有编辑器的 `cid139/path139` 字段和各自配置，目录 ID 始终作为字符串保存。显示路径不替代实际目录核验。绑定 `/小姐姐` 后直接在该目录内按原命名生成影片目录，不额外套电影或剧集根目录。没有绑定的插件分类保留待确认，原影视目录不会成为它的兜底。

## 查询处理函数

```go
pluginwasm.Serve(pluginwasm.Handlers{
    Recognition: func(i pluginwasm.Invocation) (pluginapi.Result, error) {
        call, err := pluginwasm.RecognitionContext(i)
        if err != nil { return pluginapi.Result{}, err }
        // 根据 call.Provider 与 call.File 查询插件维护的来源。
        // 仅在来源确认唯一身份后返回 matched 和真实 metadata。
        _ = call
        return pluginwasm.RecognitionReply(pluginapi.RecognitionResult{
            Status: "no_match",
            Reason: "来源尚未确认身份",
        })
    },
})
```

`metadata` 必填 `id/title/type`，`type` 为 `movie` 或 `tv`；可填 `year/season/episode/ids/release_date/studio/people/tags`。电影身份会清除文件名继承的季集信息；剧集必须确认集数。`ids.tmdb` 仅填真实 TMDB ID，插件确认后宿主不强制再查询 TMDB。

| status | 自动整理行为 |
| --- | --- |
| `matched` | 唯一确认后，按插件声明分类与现有命名整理 |
| `no_match` | 已被声明认领，仍未确认；保留待确认 |
| `not_found` | 来源未找到；保留待确认 |
| `ambiguous` | 多个候选或多个插件同时确认；保留待确认 |
| `error` | 查询或协议失败；保留待确认 |

`provider/package/file/category/naming/destination/names` 的最终值由宿主绑定或计算。插件返回的分类键、落点和来源包证据会被覆盖，文件名关键词只是认领线索，不是身份凭据。多个插件同时匹配时不会按安装顺序选赢家；唯一确认可继续，多个确认或任一查询异常会保留待确认，需要调整插件关键词或查询实现后重试。

识别阶段复用已声明的 HTTP GET、服务只读请求、私有缓存、文件读取及解析工具，不下载媒体本体。整理仍复用已有云盘驱动的改名、移动和字幕配套流程。识别生命周期不执行文件移动、发布或递归识别。宿主在调用、返回、规划和写入前重新检查启用状态、版本及当前目录授权。

## 可调用接口与兼容

- `recognition.providers` / `RecognitionProviders()`：读取处理器、关键词、分类键及启用状态，不触发网盘请求。
- `recognition.identify` / `IdentifyRecognition()`：输入 `account/id`，省略 `route` 时自动使用启用插件的声明，返回身份及宿主渲染的 `names`，不移动文件。
- `GET /api/organize/extensions`：读取已安装插件的分类声明，供现有分类策略使用；沿用网页登录保护。
- `GET /api/organize/recognition/providers`：保留处理器发现接口供已有调用者使用。
- `organize.preview/apply`：本机视频和外部 STRM 预览同样支持自动认领。输出目录由调用者的 `output_id` 选择，云盘分类绑定不替代本机输出目录。待确认项不执行，STRM 原始 URL 和文件字节保持一致。

独立 SDK 工具仍接受显式 `route` 和 `recognition`，用于已有工作流兼容或人工指定处理器：`native` 直接原生识别，`provider` 固定来源，旧的显式 `auto` 保留 `no_match` 后原生回落语义。这个兼容行为不用于原生自动整理；需要避免 TMDB 的工作流使用自动声明认领或固定 `provider`。原生整理忽略旧任务分流字段，全部跟随当前启用插件。

独立工具可继续使用 `naming.directories/file` 和 `destination: {id,path}`。模板由宿主渲染并验证路径组件，扩展名由宿主保留；本机目标必须与 `output_id` 一致。预览、执行与回执重放均复核来源包及调用者授权。

## 交互与转存

交互插件可读取清单附件、保留原分组和编号候选、调用 `transfer.submit` 转存到现有待整理目录，再等待文件真正入盘后启动原整理任务。需要单次指定目录时可用 `SubmitTransferTo`，它关闭该次原生自动整理，随后由工作流显式启动；不会修改保存的转存设置。

115 离线任务的 `success` 表示受理，返回 `submission_only=true, ready=false, stage=submitted`。插件须继续核对 `files.list/stat` 的实际对象、预期大小及可用内容哈希，之后再识别整理。其他网盘只使用其声明支持的链接协议。

本合同提供通用运行基础和开发骨架；真实站点身份查询、关键词清单及资源导入交互由独立业务插件实现和维护。
