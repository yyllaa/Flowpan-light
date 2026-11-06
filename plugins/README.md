# Flowpan 外部插件目录

在 Flowpan 的“辅助插件 → 插件管理 → 插件源”手动添加以下目录地址，然后刷新：

```text
https://github.com/yyllaa/Flowpan-light/tree/main/plugins
```

在“插件市场”选择需要的插件安装。添加来源和刷新只读取目录，不批量安装或启用插件。后续本目录新增插件后，刷新同一来源即可选择安装。

## 目录结构

```text
plugins/
  index.json           # 插件源总索引
  README.md            # 来源和目录约定
  build.py             # 当前插件和 SDK 构建入口
  dian/                # 癫影独立目录
    README.md          # 使用与构建说明
    manifest.json      # 插件清单
    go.mod             # 独立模块，引用共用 SDK
    main.go            # 插件入口
    dian_parse.go      # 站点解析
    dian_web.go        # 浏览器请求
    results.go         # 资源结果
    parse_test.go      # 回归测试
```

每个插件使用自己的目录，保存源码、清单、说明和测试；各自维护版本和发布包。共用 SDK 位于 [`sdk/plugins`](../sdk/plugins)，浏览器和 FFmpeg 由 Flowpan 统一提供。

## 可安装插件

| 插件 | 版本 | 能力 | 源码与说明 |
| --- | --- | --- | --- |
| 癫影 | 1.1.1 | 状态、签到、资源搜索、确认解锁 | [dian](dian) |

[插件与 SDK 下载](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.06)。下载包可直接在 Flowpan 本地导入。

## 新增或更新插件

1. 新插件在 `plugins/<插件目录>/` 中管理，清单使用独立、稳定的插件 ID；目录之间不共享账号配置或私有状态。
2. 构建并测试该插件的 WASM ZIP，将版本包上传到本仓库 Releases，记录包的 SHA-256 和对应源码 commit。
3. 在 `index.json` 的 `plugins` 数组中新增该插件清单，或只更新它自己的条目。索引包地址、哈希和 commit 必须与实际发布一致。
4. 用户刷新本目录来源后即可在插件市场查看和选择。已有安装的启用状态、配置和授权由宿主管理。

索引仅列出可单独安装的外部插件，宿主内置功能不列入本来源。增加插件不会要求用户再添加另一个来源地址。
