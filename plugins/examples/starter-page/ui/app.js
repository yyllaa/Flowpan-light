const root = document.querySelector("main");
const state = document.getElementById("state");
const result = document.getElementById("result");
const runButton = document.getElementById("run");
const settingsButton = document.getElementById("settings");
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
let api, ui;

function showState(title, error = false) {
  if (ui) state.replaceChildren(ui.state({ title, error }));
  else state.textContent = title;
}

async function run() {
  runButton.disabled = true;
  showState("正在运行");
  try {
    let job = await api.run("run");
    for (let n = 0; ["queued", "running"].includes(job.status) && n < 120; n++) {
      await pause(500);
      job = await api.task(job.id);
    }
    if (job.status !== "success") throw Error(job.message || "处理尚未完成，请在插件运行记录中查看");
    showState(job.result?.message || "运行完成");
    result.hidden = false;
    result.textContent = JSON.stringify(job.result?.data || {}, null, 2);
  } catch (error) { showState(error.message, true); }
  finally { runButton.disabled = false; }
}

runButton.disabled = true;
settingsButton.disabled = true;
try {
  [api, ui] = await Promise.all([window.FlowpanPage.connect(), window.FlowpanPage.loadUI()]);
  api.fitContent(root);
  const context = await api.context();
  showState(context.page.label + "已连接");
  runButton.disabled = false;
  settingsButton.disabled = false;
  runButton.addEventListener("click", run);
  settingsButton.addEventListener("click", () => api.openSettings().catch(error => showState(error.message, true)));
} catch (error) { showState(error.message, true); }
