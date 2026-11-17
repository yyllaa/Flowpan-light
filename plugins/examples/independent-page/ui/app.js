const message = document.getElementById("message"), result = document.getElementById("result"), progress = document.getElementById("progress");
let api, mediaID, running = false;
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
function showError(error) { message.textContent = error.message; message.dataset.error = "true"; }
async function run(action, input = {}) {
  if (running) return;
  running = true; message.dataset.error = "false"; message.textContent = "正在处理";
  document.querySelectorAll("button").forEach(button => button.disabled = true);
  try {
    let job = await api.run(action, input);
    // Watch the durable host job, rather than submitting repeated actions.
    for (let n = 0; ["queued", "running"].includes(job.status) && n < 120; n++) { await pause(500); job = await api.task(job.id); }
    if (job.status !== "success") throw Error(job.message || "处理尚未完成，请在插件运行记录中查看");
    const data = job.result?.data || {};
    message.textContent = job.result?.message || "处理完成";
    result.hidden = false; result.textContent = JSON.stringify(data, null, 2);
    if (data.media) {
      mediaID = data.media.id; progress.hidden = false; progress.value = data.media.progress;
      message.textContent = "媒体任务：" + data.media.status + " · " + Math.round(data.media.progress) + "%";
    }
  } catch (error) { showError(error); }
  finally { running = false; document.querySelectorAll("button").forEach(button => button.disabled = false); document.getElementById("status").disabled = !mediaID; }
}
try {
  api = await window.FlowpanPage.connect();
  const context = await api.context(); message.textContent = context.page.label + "已连接";
  document.getElementById("connect").addEventListener("click", () => run("run"));
  document.getElementById("settings").addEventListener("click", () => api.openSettings().catch(showError));
  document.getElementById("status").addEventListener("click", () => run("status", { id: mediaID }));
  document.getElementById("media-form").addEventListener("submit", event => {
    event.preventDefault(); const input = Object.fromEntries(new FormData(event.currentTarget));
    if (!input.path.trim()) { showError(Error("请选择已授权的音乐文件")); return; }
    input.key = "music-" + Date.now() + "-" + Math.random().toString(36).slice(2,10);
    void run("remux", input);
  });
} catch (error) { showError(error); }
