# 插件导航与独立页面

插件可以为项目侧栏声明快捷入口。入口统一为 /p/{插件ID}/{页面ID}，不会覆盖项目原生路由。安装且启用后出现，停用或卸载后移除。原有插件卡片、配置、动作、运行记录和授权界面继续保留。

## 清单声明

在 manifest 的 definition 中添加 pages：

    "pages": [
      {
        "id": "music",
        "label": "音乐刮削",
        "icon": "file",
        "description": "音乐信息和文件封装",
        "navigation": true,
        "entry": "ui/index.html"
      }
    ]

entry 留空时，宿主把原有声明式工作区呈现为独立页面，插件无需维护前端。填写 entry 时，宿主加载插件自己的 HTML、CSS、JavaScript、图片和字体。navigation 为 false 的页面仍可通过自己的地址访问，适合详情页。

同一个插件最多 8 个页面。页面 ID 必须唯一，名称最多 32 个字符。所有前端资源放在 ui/ 下，最多 100 个文件，每个最多 4 MiB；仅支持 html、js、css、json、svg、png、jpg、jpeg、webp、woff2。构建时把资源路径设为相对路径，例如 Vite 的 base 设为 ./。不支持通过插件前端注册宿主路由或加载远程脚本。

## 页面 SDK

导出模板：

    python tools/plugin-kit.py --out ../music-plugin --id music-service --name 音乐服务 --page

模板把页面桥接 SDK 复制到 ui/flowpan-page-sdk.js。前端首先加载它，再使用：

    const api = await window.FlowpanPage.connect();
    const context = await api.context();
    const job = await api.run("remux", {
      key: "song-01-tags-v1",
      path: "/media/music/song.flac",
      format: "flac",
      title: "歌曲",
      artist: "艺术家",
      album: "专辑"
    });
    const progress = await api.task(job.id);
    const history = await api.tasks(20, 0);
    await api.openSettings();

所有动作必须事先在 definition.actions 中声明。run 返回持久化任务记录；前端使用 task 查询执行结果，不能通过重复 run 代替查询。cancel 仅取消本插件允许取消的任务。危险动作由宿主确认窗口确认，前端传入 confirmed 不产生授权。context 只包含页面描述、动作定义和 API 版本，不包含账号、配置密钥、Cookie 或宿主登录令牌。

前端在不带 allow-same-origin 的 iframe 沙箱内运行，既不能读取宿主页面，也不能直接请求宿主 API 或站点。项目通过 MessageChannel 代理固定的动作/任务接口。插件自行维护的 Vue、React 或普通 ES 模块都可打包进 ui/；网络请求交给 WASM 的宿主工具执行，并遵守插件声明的域名及代理策略。

“插件设置”继续使用项目原来的界面，密钥只保存在宿主。不要在前端静态资源中写入任何密钥。ui/ 是公开静态内容，其余包文件、WASM、源码和私有数据不会被页面资源路由提供。

## 调用依赖与事件

浏览器、媒体工具和网盘事件沿用统一的 WASM SDK，不另建一套网页能力接口：

- 浏览器：声明 browser.use 和允许访问的域名，在动作中调用 OpenBrowser 等 SDK 方法。
- FFmpeg：声明 media.read、media.process，调用 ProbeMedia、SubmitMedia、MediaTask、MediaResult、CancelMedia。可查询 Capabilities 了解依赖状态。依赖仍在项目设置中按需下载安装。
- 音乐封装：SubmitMedia 的 operation 为 remux，使用流复制，不重新编码。支持 flac、m4a、m4b、mp3 等容器，输入编码必须与输出容器兼容。metadata 支持 title、artist、album、album_artist、genre、date、track、disc、comment；无法用它传入任意 FFmpeg 参数。
- 文件范围：输入必须位于当前插件已授权的本机目录。结果在插件私有媒体目录，源文件保持不变；相同任务键和参数复用原任务，不同参数必须使用新键。MediaResult 支持分块读取本插件的结果。
- 网盘事件：继续通过 definition.subscriptions 注册统一存储事件。新网盘驱动通过项目标准操作产生相同事件，事件授权与账号/目录范围照常生效，页面不直接操作其他网盘的数据。

示例文件在 plugins/examples/independent-page。它演示独立页面、依赖检查、封装提交和进度读取，不包含音乐元数据网站的刮削协议。

## 打包与更新

使用模板中的 build.py 或项目的 plugin pack/check 命令。打包器只包含清单、WASM 和经过校验的 ui/ 文件。前端资源随插件 ZIP 一起安装和更新，不进入核心项目编译产物。

每个页面 URL 带当前包摘要，资源逐个校验安装凭据中的哈希。插件更新后旧资源地址与旧 RPC 版本失效；页面刷新到新版本。停用和卸载会阻止后续页面动作与资源访问，侧栏自动更新。旧插件没有声明 pages 时维持原有行为。

## 复用宿主状态组件（2026-11-15）

独立页面可以使用 `FlowpanPage.loadUI()` 加载宿主公共状态组件，不依赖 Vue，也不获得任何额外业务权限。SDK 从宿主同源静态资源加载脚本与样式，沿用原页面 CSP；业务请求仍经过 `connect()` 的 MessageChannel 合同。

```javascript
const ui = await FlowpanPage.loadUI();
const root = document.querySelector("main");
root.replaceChildren(ui.status("running"), ui.progress(35, "正在生成字幕"));
// 查询失败时给出原因和明确的重试入口。
root.replaceChildren(ui.state({
  title: "字幕服务暂时离线",
  detail: "检查依赖服务后重试。",
  error: true,
  retry: () => refreshTask(),
}));
```

组件方法返回 DOM 节点，可用 `append` / `replaceChildren` 更新。`status(state, label?)` 支持 queued、running、cancelling、success、failed、attention、cancelled、enabled、disabled、unconfigured、update；`progress(number|null, label?)` 区分 0% 与未知进度；`state({title,detail,error,busy,retry})` 提供空状态、错误和重试；`save({dirty,busy,error,saved})` 显示表单状态。组件把外部文字作为纯文本插入，宿主凭据与原始 API 均不进入页面。

`loadUI()` 返回 Promise，可重复调用；加载失败后可再次调用重试。插件打包时继续只包含自己的页面与业务逻辑，公共状态组件随宿主维护。

## 文件导出

独立页面 iframe 允许脚本、表单交互事件及文件下载，继续隔离宿主源；CSP 的 `form-action none` 禁止页面发起原生表单导航。公共 `/assets/ui/` 组件允许无凭据跨源加载，业务 API 仍经父页面 RPC 授权。插件可将已授权 RPC 结果用 Blob 导出为 TXT 等本地文件。宿主确认及账号权限仍由父页面和服务端校验，页面无法读取宿主会话或本地配置。
