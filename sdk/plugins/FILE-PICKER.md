# 插件来源与目录选择

宿主功能 `ui.file-picker: 1` 提供通用来源选择与只读目录浏览。音乐刮削 1.0.3 使用此合同；其他文件处理插件可按相同声明接入，共享宿主不包含插件 ID 分支。

```json
{
  "requires": {"features": {"ui.file-picker": 1}},
  "definition": {
    "permissions": ["accounts.read", "files.read"],
    "fields": [
      {"key": "source", "label": "来源", "type": "account-list", "required": true, "max": 1},
      {"key": "folder", "label": "目录", "type": "path", "required": true,
       "extensions": {"flowpan.files": {"account_field": "source"}}}
    ]
  }
}
```

这是清单片段，需要与已有 schema_version、ID、版本及运行时等字段合并。`account-list` 的配置值仍是字符串数组；`max: 1` 只改变控件为单选，不改变数据格式。未声明此限制的旧插件继续使用多选。

宿主根据统一账号目录及插件当前授权生成选项，支持 115 的现有账号别名、139、插件网盘的稳定来源 ID 及 `local`。来源名称包含提供者，以区分同名账号。已保存的 `pool:` 账号值保留，撤销授权的来源不会继续出现在有效选项中。

目录控件调用 `GET /api/plugins/v1/{plugin_id}/directories?account={source}&id={folder}&offset=0&cursor=`。省略 id 时返回已授权目录入口；返回 current、items、has_more、next_cursor、next_offset。网盘目录 ID 为不透明字符串，不转换为整数。本机使用宿主或容器可见的绝对路径。分页沿用原文件接口及缓存，仅展示目录，文件占满一页时仍可继续加载。

每次请求复核插件权限、账号、目录及禁用工具，沿用 `files.stat` / `files.list` 的范围检查；声明不会授予账号或目录。目录浏览不创建任务、不封装或上传文件。绑定来源变化时清空旧目录，并忽略旧来源尚未结束的请求。浏览目录不自动保存配置，必须选择后再保存。

未绑定目录时先在项目中配置目录，或使用已授权范围内的目录 ID 手动填写。配置选择与刮削引擎能力不同：某个来源能否回写、备份和替换由 `files.capabilities` 的实际结果决定，不能凭显示了来源选项假定其支持全部操作。

旧宿主缺少此功能时，兼容检查会保留当前已安装版本并显示所需功能。更新一次宿主后，后续插件可复用声明，不需要为插件品牌增加代码。
