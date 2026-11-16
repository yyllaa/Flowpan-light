(function () {
  "use strict";
  const $ = id => document.getElementById(id);
  const state = { api:null, ui:null, items:[], accounts:[], total:0, offset:0, limit:30, selected:new Map(), timer:null, loading:false, generation:0, disposed:false, pending:false, busy:"" };
  const active = item => ["queued","running","cancelling"].includes(item.status);
  const bytes = size => { const units=["B","KiB","MiB","GiB","TiB"]; let i=0,n=size; while(n>=1024 && i<units.length-1){n/=1024;i++;} return (i?n.toFixed(2):String(n))+" "+units[i]; };
  const node = (tag, text, className) => { const element=document.createElement(tag); if(text!==undefined) element.textContent=text; if(className) element.className=className; return element; };
  function notice(message,error=false) { $("notice").textContent=message; $("notice").classList.toggle("error",error); $("notice").hidden=!message; }
  async function run(action,input={}) {
    let job=await state.api.run(action,input);
    for(let attempt=0;attempt<100 && ["queued","running","cancelling"].includes(job.status);attempt++) { await new Promise(resolve=>setTimeout(resolve,250)); if(state.disposed) throw Error("页面已关闭"); job=await state.api.task(job.id); }
    if(job.status!=="success" || job.result?.failed) throw Error(job.result?.message || job.message || "操作未完成，请刷新后重试");
    return job.result?.data || {};
  }
  function links() { return [...state.selected.values()].filter(item=>item.status==="success" && item.link).map(item=>item.link).join("\n"); }
  function updateSelection() {
    const text=links(); $("links").value=text; $("link-output").hidden=!text;
    $("selection-count").textContent="已选 "+state.selected.size+" 项";
    $("copy").disabled=$("export").disabled=!text; $("clear").disabled=!state.selected.size;
    $("select-page").checked=state.items.length>0 && state.items.every(item=>state.selected.has(item.id));
    $("select-page").indeterminate=!$("select-page").checked && state.items.some(item=>state.selected.has(item.id));
  }
  async function copy(text) {
    const policy=document.permissionsPolicy || document.featurePolicy;
    if(navigator.clipboard && policy?.allowsFeature("clipboard-write")) {
      try { await navigator.clipboard.writeText(text); notice("链接已复制");return; } catch { /* Use the selected text when clipboard permission is unavailable. */ }
    }
    const output=$("links"); output.value=text; $("link-output").hidden=false; output.focus(); output.select(); notice(document.execCommand("copy") ? "链接已复制" : "链接已选中，可手动复制");
  }
  async function act(action,item) {
    if(state.busy) return; state.busy=item.id; render();
    try { await run(action,{id:item.id}); if(action==="remove") state.selected.delete(item.id); notice(""); await refresh(); }
    catch(error) { notice(error.message,true); }
    finally { state.busy=""; render(); }
  }
  function render() {
    const list=$("list"); list.replaceChildren();
    if(!state.items.length) { const empty=node("div",undefined,"empty"); empty.append(node("strong",state.total ? "当前页没有记录" : "还没有生成记录"),node("p","在文件浏览中勾选 115 文件，选择“生成 ED2K”。")); list.append(empty); }
    for(const item of state.items) {
      if(state.selected.has(item.id)) state.selected.set(item.id,item);
      const row=node("article",undefined,"record"); row.classList.toggle("checked",state.selected.has(item.id));
      const check=node("input"); check.type="checkbox";check.checked=state.selected.has(item.id);check.setAttribute("aria-label","选择 "+item.name);
      check.addEventListener("change",()=>{if(check.checked) state.selected.set(item.id,item);else state.selected.delete(item.id);row.classList.toggle("checked",check.checked);updateSelection();});
      const description=node("div"); description.append(node("strong",item.name));const path=node("small",item.path,"path");path.title=item.path;description.append(path);
      const meta=node("div",undefined,"metadata");meta.append(node("span",bytes(item.size),"size"),node("span",state.accounts.find(a=>a.id===item.account)?.name || item.account,"size"));description.append(meta);
      const progress=node("div",undefined,"state");progress.append(state.ui.status(item.status,item.status==="success" ? "已生成" : item.status==="cancelled" ? "已停止" : undefined));
      if(active(item)) progress.append(state.ui.progress(item.size ? Math.min(100,item.done/item.size*100) : 0,bytes(item.done)+" / "+bytes(item.size)));
      if(["paused","cancelled"].includes(item.status)) description.append(node("small",item.message));
      if(item.retry_at) description.append(node("small","冷却至 "+new Date(item.retry_at*1000).toLocaleTimeString("zh-CN",{hour:"2-digit",minute:"2-digit"})));
      const actions=node("div",undefined,"actions");
      const button=(label,handler,danger=false)=>{const control=node("button",label,danger ? "danger" : "");control.type="button";control.disabled=!!state.busy;control.addEventListener("click",handler);actions.append(control);};
      if(item.link) button("复制",()=>copy(item.link));
      if(["paused","cancelled"].includes(item.status)) button("继续",()=>act("retry",item));
      if(["queued","running"].includes(item.status)) button("停止",()=>act("cancel",item));
      if(!active(item)) button("删除",()=>act("remove",item),true);
      row.append(check,description,progress,actions);list.append(row);
    }
    $("total").textContent=state.total+" 条记录";$("page").textContent=String(Math.floor(state.offset/state.limit)+1);
    $("previous").disabled=state.offset===0||state.loading;$("next").disabled=state.offset+state.limit>=state.total||state.loading;
    updateSelection();
  }
  async function refresh(background=false) {
    if(!state.api || state.disposed) return; if(state.loading) { if(!background) { state.pending=true; state.generation++; } return; } state.loading=true;clearTimeout(state.timer);const generation=++state.generation;
    try {
      const data=await run("list",{account:$("account").value,status:$("status").value,query:$("query").value.trim(),offset:state.offset,limit:state.limit});
      if(state.disposed || generation!==state.generation) return;
      const page=data.page;state.items=page.items;state.total=page.total;state.offset=page.offset;
      if(data.accounts) { state.accounts=data.accounts;const selected=$("account").value;$("account").replaceChildren(new Option("全部账号",""),...data.accounts.map(account=>new Option(account.name,account.id)));$("account").value=selected; }
      if(!background) notice("");render();
    } catch(error) { if(!state.disposed && generation===state.generation) notice(error.message,true); }
    finally { state.loading=false;$("previous").disabled=state.offset===0;$("next").disabled=state.offset+state.limit>=state.total;if(state.pending && !state.disposed) { state.pending=false;refresh(); } else if(!state.disposed && state.items.some(active)) state.timer=setTimeout(()=>{if(!document.hidden) refresh(true);},5000); }
  }
  $("filters").addEventListener("submit",event=>{event.preventDefault();state.offset=0;refresh();});
  for(const id of ["account","status"]) $(id).addEventListener("change",()=>{state.offset=0;refresh();});
  $("previous").addEventListener("click",()=>{state.offset=Math.max(0,state.offset-state.limit);refresh();});$("next").addEventListener("click",()=>{state.offset+=state.limit;refresh();});
  $("select-page").addEventListener("change",()=>{for(const item of state.items){if($("select-page").checked) state.selected.set(item.id,item);else state.selected.delete(item.id);}render();});
  $("clear").addEventListener("click",()=>{state.selected.clear();render();});$("copy").addEventListener("click",()=>copy(links()));
  $("export").addEventListener("click",()=>{const url=URL.createObjectURL(new Blob(["\ufeff"+links()+"\n"],{type:"text/plain;charset=utf-8"}));const anchor=node("a");anchor.href=url;anchor.download="ED2K-"+new Date().toLocaleDateString("sv-SE")+".txt";document.body.append(anchor);anchor.click();anchor.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);});
  document.addEventListener("visibilitychange",()=>{if(!document.hidden) refresh(true);});
  window.addEventListener("pagehide",()=>{state.disposed=true;state.generation++;clearTimeout(state.timer);},{once:true});
  Promise.all([FlowpanPage.connect(),FlowpanPage.loadUI()]).then(([api,ui])=>{state.api=api;state.ui=ui;refresh();}).catch(error=>notice(error.message,true));
})();
