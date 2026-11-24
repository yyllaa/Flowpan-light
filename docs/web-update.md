# Web 独立更新

Web 更新替换 Flowpan 的后台界面，使用当前主程序提供的 API。支持独立 Web 更新的镜像可在运行中更新和回退。

## 在 Flowpan 里更新

1. 打开 **其他设置 → 系统更新 → Web 更新源**。支持自动检查的版本在打开此页时读取可用版本和发布说明，也可以点击 **检查更新** 手动刷新。
2. 默认使用 `yyllaa/Flowpan-light` 的 GitHub Releases，官方签名公钥已内置。使用自定义来源时在 **高级设置** 保存该来源和对应公钥。
3. 点击 **更新 Web** 安装可用版本。下载、包签名、文件完整性和后端兼容性校验均通过后，刷新页面即可使用新版。旧版界面使用 **检查并更新 Web** 按钮。

在页面看不到 Web 更新入口时，请先安装支持独立 Web 更新的主程序版本。主程序和插件各有自己的更新入口。相关操作见[升级与维护](maintenance.md)。

## 保留与回退

独立 Web 存在 `/config/updates/web/`。重建 Docker 容器时沿用原 `/config` 挂载。常规启动从通过校验且兼容的本地包与镜像内置 Web 中选择较新版本，同版优先使用内置 Web。启动自检可离线完成。

Web 更新保留当前版本和一个回退版本。可在 Web 更新源点击 **回退到** 对应版本。手动回退在重启后仍生效。回退到较旧的本地包后，换用带有较新内置 Web 的镜像会重新按版本选择。

若 Web 页面打不开，可在同一 Flowpan 地址下访问 `/web-recovery`，用当前浏览器加载镜像内置页面，登录后回退，再点击 **退出恢复模式**。

## 发布位置

正式 Web 更新使用 `web-api<代数>-<版本>` 标签，在 [GitHub Releases](https://github.com/yyllaa/Flowpan-light/releases) 上传签名的 `flowpan-web.zip`。Release 正文填写更新说明。Flowpan 从同一 Web API 代数中读取附带 Web ZIP 的正式版本。Web 版本号和主程序版本号分别维护。

仓库内的插件页面桥接脚本由 [插件 SDK](../sdk/plugins/README.md) 管理。官方 Web 的安装包与更新说明统一从 Releases 获取。
