/* Flowpan plugin page bridge, API v1. No session, secret or raw host API is exposed. */
(function () {
  "use strict";
  let connection;
  function connect() {
    if (connection) return connection;
    connection = new Promise((resolve, reject) => {
      const timeout = setTimeout(() => { window.removeEventListener("message", ready); connection = null; reject(Error("插件页面连接超时")); }, 15000);
      function ready(event) {
        if (event.source !== window.parent || event.data?.type !== "flowpan.page.connected" || event.data?.version !== 1 || !event.ports[0]) return;
        clearTimeout(timeout); window.removeEventListener("message", ready);
        const port = event.ports[0], pending = new Map();
        let sequence = 0, fitObserver, fitRoot, fitFrame, lastHeight;
        const measure = () => {
          cancelAnimationFrame(fitFrame);
          fitFrame = requestAnimationFrame(() => {
            const height = Math.ceil(fitRoot.getBoundingClientRect().height);
            if (height >= 32 && height !== lastHeight) { lastHeight = height; port.postMessage({ type: "flowpan.page.resize", height }); }
          });
        };
        const fitContent = element => {
          if (!(element instanceof Element)) throw Error("请选择页面内容元素");
          fitObserver?.disconnect(); fitRoot = element; lastHeight = null;
          fitObserver = new ResizeObserver(measure); fitObserver.observe(element); measure();
        };
        port.onmessage = event => {
          const message = event.data;
          if (message?.type === "flowpan.page.viewport") {
            if (Number.isFinite(message.height) && message.height >= 260 && message.height <= 50000) document.documentElement.style.setProperty("--flowpan-page-max-height", message.height + "px");
            return;
          }
          const item = pending.get(message?.id);
          if (!item) return;
          pending.delete(message.id); clearTimeout(item.timeout);
          if (message.error) item.reject(Error(message.error)); else item.resolve(message.data);
        };
        port.start();
        const call = (method, params = {}) => new Promise((resolve, reject) => {
          if (pending.size >= 8) { reject(Error("请等待当前插件请求完成")); return; }
          const id = String(++sequence);
          const message = { id, method, params };
          if (JSON.stringify(message).length > 60000) { reject(Error("插件页面参数过长")); return; }
          const timeout = setTimeout(() => { pending.delete(id); reject(Error("插件页面请求超时")); }, 120000);
          pending.set(id, { resolve, reject, timeout }); port.postMessage(message);
        });
        window.addEventListener("pagehide", () => { fitObserver?.disconnect(); cancelAnimationFrame(fitFrame); port.close(); for (const item of pending.values()) { clearTimeout(item.timeout); item.reject(Error("插件页面已关闭")); } pending.clear(); }, { once: true });
        resolve(Object.freeze({
          fitContent,
          context: () => call("context"),
          run: (action, input = {}) => call("actions.run", { action, input }),
          tasks: (limit = 20, offset = 0) => call("tasks.list", { limit, offset }),
          task: id => call("tasks.get", { id }),
          cancel: id => call("tasks.cancel", { id }),
          openSettings: () => call("settings.open"),
        }));
      }
      window.addEventListener("message", ready);
      window.parent.postMessage({ type: "flowpan.page.ready", version: 1 }, "*");
    });
    return connection;
  }
  let uiLoading;
  function loadUI() {
    if (window.FlowpanUI?.dom) { window.FlowpanUI.dom.styles(); return Promise.resolve(window.FlowpanUI.dom); }
    if (!uiLoading) uiLoading = new Promise((resolve,reject) => {
      const script=document.createElement('script'); script.src='/assets/ui/host-ui.js?v=1';
      script.onload=() => { if (!window.FlowpanUI?.dom) { uiLoading=null; reject(Error('公共组件加载失败')); return; } window.FlowpanUI.dom.styles(); resolve(window.FlowpanUI.dom); };
      script.onerror=() => { script.remove(); uiLoading=null; reject(Error('公共组件加载失败，请重试')); };
      document.head.append(script);
    });
    return uiLoading;
  }
  window.FlowpanPage = Object.freeze({ connect, loadUI });
})();
