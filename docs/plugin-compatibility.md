# 插件与主程序的兼容合同

插件可以独立更新算法、来源、规则、动作和页面。主程序提供文件、媒体、网络、状态、调度等通用原语。插件在清单中声明依赖，主程序在执行前检查实际支持情况。

从 `2026.11.17-plugin-contract` 起，主程序稳定支持 `requires`、`optional` 和带命名空间的 `extensions`。旧版主程序需要先更新到这一基础版本；只升级插件不会给旧解析器增加字段。

## 清单声明

```json
{
  "min_host_version": "1",
  "requires": {
    "api_version": "1",
    "features": {"jobs.continuation": 1, "state.indexed-get": 1},
    "tools": ["kv.get", "kv.set", "http.fetch"],
    "limits": {"continuation_seconds": 300}
  },
  "optional": {"features": {"media.cover-artifact": 1}},
  "extensions": {"example.music.rules": {"keywords": ["现场", "原声带"]}}
}
```

此片段放在现有插件清单根对象，保留原有身份、版本、运行模块及权限。`requires.tools` 对应工具所需权限也必须列入清单 `permissions`。`min_host_version: "1"` 保留旧合同中的 API 级别含义；新插件优先声明功能和工具，不按日期构建号绑定主程序。

- `requires`：必需依赖，缺少任何一项就拒绝安装、启用或运行，并说明名称、需要值和实际值。
- `optional`：可选依赖，返回缺项但允许安装；插件检测后执行降级分支。
- `features`：功能名及所需的最低整数版本。同名功能增加新语义时由主程序提高版本。
- `tools`：插件实际调用的工具名称，名字精确匹配。工具存在与账号、目录授权是两层检查。
- `limits`：插件需要的最小运行上限；例如单次分段允许等待多久、封面最大字节数。
- `extensions`：插件自定义数据，主程序保留内容并纳入包完整性校验。根清单、definition、配置字段和动作支持此容器。键必须带命名空间，例如 `example.music.rules`；每个容器最多 32 项、64 KiB。普通未知字段仍会报错，拼错字段不会被静默忽略。

`extensions` 中的数据由插件自己的代码消费。填写一个名字不会增加系统权限、宿主工具或自动执行某项行为。

## 查询与降级

市场快照 `/api/plugins/v1/market` 提供 `host_contract`，目录条目和已安装条目提供 `compatibility`，其中含 `compatible`、`missing` 和 `optional_missing`。能力入口 `/api/plugins/v1/capabilities` 和 CLI `plugin capabilities` 返回 `contract`。

WASM 内使用 SDK：

```go
report, err := pluginwasm.CheckRequirements(
    &pluginapi.HostRequirements{Tools: []string{"kv.get", "kv.set"}},
    &pluginapi.HostRequirements{Features: map[string]int{"media.cover-artifact": 1}},
)
if err != nil {
    return err
}
coverEnabled := len(report.OptionalMissing) == 0
_ = coverEnabled // 用于插件自身的封面降级分支
```

SDK 请求 `host.capabilities`。运行器返回实际绑定的工具；旧宿主没有新合同字段时，SDK只复用可证实的工具和上限，不推测新功能。实际调用仍检查权限、账号与路径范围、确认要求及依赖状态。

市场先展示不兼容条目及缺项；安装入口在下载与暂停旧插件之前检查。包导入先读稳定依赖信封，再严格解析完整清单。因此插件声明未来功能并携带未来定义时，会先给出兼容性原因。解包后再次检查，防止目录声明和实际包不同。启用、健康检查和运行也复核。

## 哪些改动由插件独立发布

| 功能 | 插件使用的公共基础 | 主程序是否需要随插件业务升级 |
|---|---|---|
| 新音源、识别规则、关键词、匹配或分类策略 | HTTP、状态、识别/整理扩展点 | 已有原语满足时无需更新 |
| 新配置、按钮、动作、定时任务、仪表盘与独立页面 | definition、actions、schedule、dashboard、UI SDK | 使用已有字段和组件时无需更新 |
| 本地/网盘文件扫描、暂存、封装、上传及恢复 | files、artifacts、media、tasks | 使用已有操作与格式时无需更新 |
| 新网盘提供者 | storage provider 合同、账号与范围授权 | 提供者实现现有接口时无需增加品牌分支 |
| 插件间调用与结果复用 | provides / uses、capabilities.call / submit、接口版本与持久绑定 | 同一接口版本下插件独立升级；详见 plugin-capabilities.md |
| 新的系统设备、协议原语或宿主字段语义 | 新宿主实现与功能版本 | 先补通用基础，再由插件声明依赖 |

FFmpeg、FFprobe、浏览器等外部依赖使用现有依赖状态和下载流程。WASM 插件通过公共媒体与浏览器工具调用，不自行在宿主执行 shell 或安装任意原生模块。请求间隔、来源协议、查询缓存和批次策略由插件维护；主程序继续处理任务持久化、账号级调度、文件校验与权限。

## 已提供的语义功能版本

全部初始版本为 `1`：`manifest.requirements`、`manifest.extensions`、`plugins.capabilities`、`jobs.continuation`、`http.response.base64`、`media.cover-artifact`、`media.lyrics`、`state.indexed-get`、`files.verified-publish`、`ui.pages`、`actions.dynamic-options`、`tasks.activity`。完整工具及限制以运行时返回为准，开发时不要复制一份硬编码列表。

## 兼容发布检查

发布者使用数字版本、`release.notes`、`plugin pack --release` 和 `plugin check --release`。有新依赖就在 `requires` 声明；可选功能先写好实际降级分支再放 `optional`。同一主程序下连续安装新插件版本，验证配置、状态、定时任务、健康检查与插件回退。主程序自身新增功能时保留旧字段语义，集中扩展公共合同，避免按插件 ID 增加分支。
