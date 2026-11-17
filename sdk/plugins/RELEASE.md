# 插件独立发布与固定宿主验收

业务逻辑、来源协议和规则由插件维护。发布插件时使用已发布的主程序，不编译或修改主程序来配合插件。本仓库的 `plugins/host-baseline.json` 固定验收镜像 ID、能力快照和 SDK 源码哈希；相同能力名称的历史实现差异先由固定构建验收，不能把“工具存在”当作所有行为均已验证。

## 声明检查

```powershell
python tools/plugin_release.py check --all
python tools/plugin_release.py test-modules
python -m unittest discover -s tools/tests -p test_plugin_release.py
```

新 WASM 插件必须声明 `requires.api_version`、实际工具、配置及页面所需功能。检查器使用 Go AST 解析直接工具调用、SDK 别名和 SDK 封装的传递调用，检查清单、权限和固定宿主是否满足要求。可选能力在 `optional` 中声明，由插件实现降级。动态派发位置单独报告，仍须编写工作流测试；静态分析不会证明全部动态分支已运行。

原生内置配置继续使用原合同。根 CI 另行发现 `plugins/`、`sdk/` 中的独立 Go 模块并进入各模块运行 build、vet、test，不能只在根目录执行 `./plugins/...`。

## 导出开发包

```powershell
python tools/plugin-kit.py --out ../my-plugin --id my-plugin --page
```

开发包包含固定基线、发布工具及 `SDK.lock.json`。模板自动列出实际所用依赖。构建时核对 SDK 文件哈希；新增宿主原语或改变 SDK 合同时，先完成对应版本的基线验收，不通过手工刷新哈希把未验证变化混入旧基线。业务插件变化不需要修改该基线。

## 构建候选包

```powershell
python tools/plugin_release.py build --plugin plugins/flowpan.ed2k --out output/my-release/ed2k.zip
```

默认使用本机已有的固定 Linux 镜像，以不可变 ID 运行，不拉取替代镜像、不访问外网。构建只编译插件 WASM；打包与 `plugin check --release` 在固定主程序中执行。机器需预先导入对应发布镜像。

没有 Docker 时，可传 `--host /path/to/flowpan --host-sha256 实际程序SHA256`。程序字节哈希及返回的完整能力快照均需匹配；二进制哈希记入候选包回执。这用于明确固定的同版二进制，不把任意开发版视为支持基线。

成功后得到 ZIP 和 `.build.json`，其中 `static_checked=true`、`runtime_verified=false`。已有输出不覆盖，版本不同放进新的发布目录。静态检查不执行插件业务，不能据此直接发布。

## 工作流验证与目录生成

```powershell
python tools/plugin_release.py verify --package output/my-release/ed2k.zip -- python tests/my_plugin_workflow.py
python tools/plugin_release.py catalog --package output/my-release/ed2k.zip --url https://example.com/ed2k.zip --out output/my-release/index.json
```

`verify` 执行真实测试命令，为测试提供 `FLOWPAN_PLUGIN_ZIP`、`FLOWPAN_IMAGE_ID`、`FLOWPAN_BASELINE`。测试应启动该固定镜像、安装该最终 ZIP，覆盖插件实际工作流；不能用占位命令、仅编译或健康检查代替。测试失败不生成新验证回执；ZIP 变化后原回执失效。

通用验收覆盖：本地 ZIP 与在线目录导入、启用、配置与数据保留、账号隔离、无 STRM/FUSE 绑定的目录选择、游标分页、权限撤销、任务进度、重启恢复、取消及回退。插件业务测试再覆盖实际处理、重复请求和异常回执。用模拟提供者做组合测试；真实网盘保留小样本和请求间隔，模拟 3000 首不写成真实 3000 首上传。

`catalog` 必须读取与当前包、SDK、固定镜像一致的 `.verified.json`。它从最终 ZIP 中读取清单并计算包 SHA-256，生成索引条目，不再手写两份清单。回执是本地/CI 测试证据，不是来自插件或第三方的可信签名；审阅测试代码与保管发布权限仍由项目负责。

## 宿主变更边界

- 现有工具能完成的需求，在插件或现有能力提供者中实现。
- 通用实现有缺陷，先给出最小复现，再修共享实现并回归现有插件。
- 确实缺少新的底层原语，才集中发布新主程序和对应语义版本；不为插件品牌添加分支。
- 癫影私有 SDK 与 ED2K 原生引擎继续按既有位置维护；是否独立迁移单独验收，不混入普通插件更新。

后续开发任务可复用：

```text
基于 plugins/host-baseline.json 中固定的已发布主程序开发插件。
业务变化在插件中实现，补齐 requires/optional，保留旧数据和可选降级。
运行声明检查、独立模块测试，构建最终 ZIP，并在固定镜像上验证完整工作流。
验证后从最终 ZIP 生成索引，保留包、SDK、主程序哈希与测试命令。
若需要修改主程序，先提供已有工具不能满足需求的具体证据。
```
