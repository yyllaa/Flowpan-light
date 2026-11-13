# 源码来源

`dian_parse.go` 的通用资源解析最初提取自用户提供的 115-Station `internal/api/dian.go`，原始 SHA-256 记录在 upstream-sha256.json。

当前版本仅通过 Flowpan 的公开宿主合同调用 `sdk.call`。此目录不分发私有站点 SDK、密封身份、开发者凭据或用户 Key。业务动作、资源结果、缓存及提交回执由本插件维护，私有协议由项目宿主提供。
