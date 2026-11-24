# 插件独立发布与固定宿主验收

业务逻辑、来源协议和规则由插件维护。发布插件时使用已发布的主程序，不编译或修改主程序来配合插件。本仓库的 `plugins/host-baseline.json` 固定验收镜像 ID、能力快照和 SDK 源码哈希；相同能力名称的历史实现差异先由固定构建验收，不能把“工具存在”当作所有行为均已验证。

## 2026-11-24 基线更新

当前验收基线为 `2026.11.23-openlist-plugin`，固定镜像 ID、能力快照和 SDK 哈希记录在 `plugins/host-baseline.json`。

本次增加 `storage.account-endpoint: 1` 和 `storage.path-refs: 1`，支持账号配置的服务端点边界及持久路径引用。OpenList 插件需要这两个合同；旧版主程序须先更新到配套镜像。已有插件继续按原依赖声明检查。

## 声明检查

在公开仓库根目录运行。

```powershell
python tools/plugin_release.py check --plugin plugins/ed2k
python tools/plugin_release.py test-modules
python -m unittest discover -s tools/tests -p test_plugin_release.py
```

新 WASM 插件必须声明 `requires.api_version`、实际工具、配置及页面所需功能。检查器使用 Go AST 解析直接工具调用、SDK 别名和 SDK 封装的传递调用，检查清单、权限和固定宿主是否满足要求。可选能力在 `optional` 中声明，由插件实现降级。动态派发位置单独报告，仍须编写工作流测试；静态分析不会证明全部动态分支已运行。

将 plugins/ed2k 换成所维护的插件目录。旧版癫影浏览器包继续使用原合同。仓库 CI 另行发现 `plugins/`、`sdk/` 中的独立 Go 模块并进入各模块运行 build、vet、test，不能只在根目录执行 `./plugins/...`。

## 导出开发包

```powershell
python tools/plugin-kit.py --out ../my-plugin --id my-plugin --page
```

开发包包含固定基线、发布工具及 `SDK.lock.json`。模板自动列出实际所用依赖。构建时核对 SDK 文件哈希；新增宿主原语或改变 SDK 合同时，先完成对应版本的基线验收，不通过手工刷新哈希把未验证变化混入旧基线。业务插件变化不需要修改该基线。

### 在独立开发包中构建和发布

以下命令在导出的插件目录执行。使用 Go 1.25、Python 3（仓库 CI 使用 3.12）和支持 Linux 容器的 Docker。用 `docker load -i` 导入单独交付的 Flowpan 主程序镜像，再用 `docker image inspect` 确认镜像 ID 和平台与 `plugins/host-baseline.json` 的 `image_id`、`platform` 一致。构建工具按该 ID 使用本机镜像。

修改 `plugin/main.go`、`manifest.json` 和 `CHANGELOG.md` 后运行以下命令。

```sh
go test ./...
python tools/plugin_release.py check --plugin .
python build.py
```

`plugin.zip` 和 `plugin.zip.build.json` 是候选包及静态构建回执。编写可重复执行的 `tests/workflow.py`，启动固定镜像并安装同一个 ZIP，再断言插件的启用、配置、运行、更新及业务异常等行为。固定基线 `2026.11.23-openlist-plugin` 上首次安装后需要手动启用。支持自动启用的新版宿主另行验收。

```sh
python tools/plugin_release.py verify --package plugin.zip -- python tests/workflow.py
python tools/plugin_release.py catalog --package plugin.zip --url https://github.com/OWNER/REPO/releases/download/TAG/my-plugin.zip --commit COMMIT_SHA_40_HEX --out plugin-index.json
```

将 URL 和 commit 换成实际的下载地址和 40 位小写源码提交 SHA。在线安装使用与插件源同一 GitHub 仓库的附件地址。`catalog` 输出当前插件的单条索引，将该条目合并到插件源的 `plugins/index.json` 后发布。每个新版本更新清单的两层版本和 `release.notes`，并在新目录生成和验证候选包。

## 构建候选包

```powershell
python tools/plugin_release.py build --plugin plugins/ed2k --out output/my-release/ed2k.zip
```

开发机编译插件 WASM。默认使用本机的固定 Linux 镜像执行 `plugin pack --release` 与 `plugin check --release`，运行参数包含 `--pull=never` 和 `--network none`。

使用对应版本的本机可执行文件构建候选包时，传入 `--host /path/to/flowpan --host-sha256 SHA256`。工具比较文件哈希和传入值，并对照固定基线检查完整能力快照。候选包回执记录该二进制的哈希。正式发布继续在固定基线上运行工作流验收。

成功后得到 ZIP 和 `.build.json`，其中 `static_checked=true`、`runtime_verified=false`。已有输出不覆盖，版本不同放进新的发布目录。静态检查不执行插件业务，不能据此直接发布。

## 工作流验证与目录生成

```powershell
python tools/plugin_release.py verify --package output/my-release/ed2k.zip -- python tests/my_plugin_workflow.py
python tools/plugin_release.py catalog --package output/my-release/ed2k.zip --url https://github.com/OWNER/REPO/releases/download/TAG/ed2k.zip --commit COMMIT_SHA_40_HEX --out output/my-release/plugin-index.json
```

`verify` 执行真实测试命令，为测试提供 `FLOWPAN_PLUGIN_ZIP`、`FLOWPAN_IMAGE_ID`、`FLOWPAN_BASELINE`。测试应启动该固定镜像、安装该最终 ZIP，覆盖插件实际工作流；不能用占位命令、仅编译或健康检查代替。测试失败不生成新验证回执；ZIP 变化后原回执失效。

通用验收覆盖：本地 ZIP 与在线目录导入、启用、配置与数据保留、账号隔离、无 STRM/FUSE 绑定的目录选择、游标分页、权限撤销、任务进度、重启恢复、取消及回退。插件业务测试再覆盖实际处理、重复请求和异常回执。用模拟提供者做组合测试；真实网盘保留小样本和请求间隔，模拟 3000 首不写成真实 3000 首上传。

主程序维护者在主程序工程运行 `npm run check:plugin-contract`。该检查包含普通插件无 STRM/FUSE 绑定时的整盘授权、115 默认／新增主号及号池隔离、授权模式保存、撤权与冻结任务、未缓存新目录的文件事件，以及网盘提供者和本机目录隔离。普通资源／动作插件只获得当前插件已勾选账号的授权；实际处理目录由插件自己的配置决定，不修改 STRM、FUSE 或其他插件的范围。固定宿主镜像上的插件工作流也须核对这些行为；旧镜像仍有共用缺陷时，先验收并发布修复宿主，不在插件中增加绕过分支。

`catalog` 必须读取与当前包、SDK、固定镜像一致的 `.verified.json`。它从最终 ZIP 中读取清单并计算包 SHA-256，生成索引条目，不再手写两份清单。回执是本地/CI 测试证据，不是来自插件或第三方的可信签名；审阅测试代码与保管发布权限仍由项目负责。

## 宿主变更边界

- 插件的配置、授权、安装、更新、启停和卸载不得改写主程序的 STRM、FUSE、整理、转存、账号及调度配置；主程序原生功能不得依赖普通插件是否启用。管理流程验收需对比前后的运行配置、持久配置、账号记录、STRM 任务、FUSE 文件及其他插件授权。插件声明并获授权的宿主工具按其操作合同执行，存储提供者继续只管理自身来源。
- 现有工具能完成的需求，在插件或现有能力提供者中实现。
- 通用实现有缺陷，先给出最小复现，再修共享实现并回归现有插件。
- 确实缺少新的底层原语，才集中发布新主程序和对应语义版本；不为插件品牌添加分支。
- 癫影私有 SDK 与 ED2K 原生引擎继续按既有位置维护；是否独立迁移单独验收，不混入普通插件更新。

后续开发任务可复用：

```text
基于 plugins/host-baseline.json 中固定的已发布主程序开发插件。
业务变化在插件中实现，补齐 requires/optional，保留旧数据和可选降级。
运行声明检查、独立模块测试，构建最终 ZIP，并在固定镜像上验证完整工作流。
在主程序工程运行 npm run check:plugin-contract，验证插件账号授权独立于 STRM/FUSE 配置、账号隔离、撤权及新目录事件；普通插件复用宿主整盘授权。
验证插件管理操作前后主程序配置、任务、账号及其他插件授权保持原值，主程序原生功能不依赖普通插件启停。
验证后从最终 ZIP 生成索引，保留包、SDK、主程序哈希与测试命令。
若需要修改主程序，先提供已有工具不能满足需求的具体证据。
```
