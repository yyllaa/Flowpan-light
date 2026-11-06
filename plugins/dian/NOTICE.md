# 源码来源

`dian_parse.go`、`dian_web.go` 的解析和浏览器请求协议提取自用户提供的 115-Station 源码 `internal/api/dian.go`、`internal/api/dianweb.go`。原始 SHA-256 保存在 `upstream-sha256.json`。

站点业务、签名、签到、搜索、解锁、搜索缓存和提交回执均由本外部 WASM 插件维护。插件调用 Flowpan 通用 `browser.open/evaluate/close`，浏览器由项目统一提供。RE0 的源码、配置和调度不迁移。
