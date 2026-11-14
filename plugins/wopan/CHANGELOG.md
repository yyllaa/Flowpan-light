# 联通云盘插件更新日志

## 0.2.2 · 2026-11-14

- 修复应用可写 FUSE 配置时报 `storage operation is not supported: delete, move`。
- 实现个人空间文件和目录删除至供应商回收站，以及移动至目标目录；文件移动携带当前文件版本。
- 校验文件与目录类型、根目录、失效引用和自身或子目录移动；未知写回执不自动重放。
- 配套主程序修复移动后来源引用、目录子树与 inode 映射的同步，保持已打开句柄可读。

配套主程序 `2026.11.14-plugin-updates` 继续包含上述 FUSE 修复，并增加插件新版提示、应用内更新日志、插件 API 仪表盘统计和按真实提供者显示的 FUSE 日志。插件维持 0.2.2，已安装此版本时更新主程序即可获得这些宿主功能。

发布包及校验值见 [GitHub Release](https://github.com/yyllaa/Flowpan-light/releases/tag/plugins-2026.11.14-wopan-fuse)。
