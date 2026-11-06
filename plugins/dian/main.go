// Build this adapter as WASI. Site logic stays in the external plugin.
package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type host interface{ Call(string, any, any) error }
type wasmHost struct{}

func (wasmHost) Call(method string, p any, out any) error { return pluginwasm.Call(method, p, out) }

type pluginPage struct {
	h       host
	session string
}

func (p *pluginPage) eval(script string, args map[string]any, secrets map[string]string, out any) error {
	return p.h.Call("browser.evaluate", map[string]any{"session": p.session, "script": script, "args": args, "secrets": secrets}, out)
}

type pageResponse struct {
	Status int    `json:"status"`
	Text   string `json:"text"`
}

func (p *pluginPage) request(method, path string, body any) (pageResponse, error) {
	var result pageResponse
	var secrets map[string]string
	if method == "POST" && path == "/auth/login" {
		secrets = map[string]string{"password": "password"}
	}
	err := p.eval(`({method,path,body,password}) => window.__s115dian(method,path,password ? {...body,password} : body)`, map[string]any{"method": method, "path": path, "body": body}, secrets, &result)
	return result, err
}

type receipt struct {
	Pending bool             `json:"pending"`
	Until   int64            `json:"until"`
	Result  pluginapi.Result `json:"result"`
}

func execute(h host, r pluginapi.Request, pause func(time.Duration)) (pluginapi.Result, error) {
	action := r.ActionID
	if action != "status" && action != "checkin" && action != "search" && action != "unlock" {
		return pluginapi.Result{}, fmt.Errorf("插件动作不存在")
	}
	email, _ := r.Config["email"].(string)
	email = strings.TrimSpace(email)
	if email == "" {
		return pluginapi.Result{}, fmt.Errorf("请配置癫影账号")
	}
	if r.TaskID == "" {
		return pluginapi.Result{}, fmt.Errorf("缺少后台任务编号")
	}
	kind, _ := r.Input["media_type"].(string)
	rawID := fmt.Sprint(r.Input["tmdb_id"])
	if number, ok := r.Input["tmdb_id"].(float64); ok {
		// JSON numbers arrive as float64; %v switches to exponent form at 1e6.
		rawID = strconv.FormatFloat(number, 'f', -1, 64)
	}
	id, idErr := strconv.Atoi(rawID)
	resourceID, _ := r.Input["resource_id"].(string)
	if action == "search" && ((kind != "movie" && kind != "tv") || idErr != nil || id <= 0) {
		return pluginapi.Result{}, fmt.Errorf("请选择电影/剧集并填写 TMDB ID")
	}
	if action == "unlock" {
		if !r.Confirmed {
			return pluginapi.Result{}, fmt.Errorf("请先确认资源解锁")
		}
		if _, _, ok := parseDianRef(resourceID); !ok {
			return pluginapi.Result{}, fmt.Errorf("资源编号无效")
		}
	}
	now := time.Now()
	key := ""
	ttl := int64(0)
	switch action {
	case "search":
		key = fmt.Sprintf("search-%s-%s-%d", accountKey(email)[:24], kind, id)
		ttl = 600
	case "checkin":
		key = "checkin-" + accountKey(email)[:24] + "-" + now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("20060102")
		ttl = 86400
	case "unlock":
		key = "unlock-" + accountKey(email)[:24] + "-" + accountKey(resourceID)[:16]
		ttl = 86400 * 30
	}
	if key != "" {
		var previous receipt
		if err := h.Call("kv.get", map[string]any{"key": key}, &previous); err != nil {
			return pluginapi.Result{}, err
		}
		if previous.Pending {
			return pluginapi.Result{}, fmt.Errorf("上次操作结果待核验，请先在站点检查，避免重复提交")
		}
		if previous.Until > now.Unix() {
			return previous.Result, nil
		}
	}
	step := func(message string) { _ = h.Call("task.progress", pluginapi.Progress{Message: message}, nil) }
	step("打开癫影")
	site := "https://m.dian115.com/"
	// A fixture manifest may explicitly authorize a loopback site for protocol tests.
	if v, _ := r.Config["site_url"].(string); v != "" {
		site = v
	}
	var opened struct {
		Session string `json:"session"`
	}
	if err := h.Call("browser.open", map[string]any{"url": site, "profile": accountKey(email)}, &opened); err != nil {
		return pluginapi.Result{}, err
	}
	defer h.Call("browser.close", map[string]any{"session": opened.Session}, nil)
	p := &pluginPage{h: h, session: opened.Session}
	ready := false
	for n := 0; n < 45; n++ {
		var state struct {
			Ready     string `json:"ready"`
			Title     string `json:"title"`
			Challenge bool   `json:"challenge"`
		}
		if err := p.eval(`() => ({title:document.title,ready:document.readyState,challenge:!!document.querySelector('#challenge-running,#challenge-stage,form[action*="challenge-platform"]')})`, nil, nil, &state); err != nil {
			return pluginapi.Result{}, err
		}
		if state.Ready != "loading" && !state.Challenge && !strings.Contains(strings.ToLower(state.Title), "just a moment") {
			ready = true
			break
		}
		pause(500 * time.Millisecond)
	}
	if !ready {
		return pluginapi.Result{}, fmt.Errorf("站点验证未完成，请检查浏览器状态")
	}
	if err := p.eval(dianSignJS, nil, nil, nil); err != nil {
		return pluginapi.Result{}, fmt.Errorf("初始化站点连接失败")
	}
	c := &dianCaller{p: p}
	step("确认登录状态")
	me, err := c.ensureLogin(&dianCfg{Email: email})
	if err != nil {
		return pluginapi.Result{}, err
	}
	pending := func() error {
		if action != "checkin" && action != "unlock" {
			return nil
		}
		return h.Call("kv.set", map[string]any{"key": key, "value": receipt{Pending: true}}, nil)
	}
	var data map[string]any
	switch action {
	case "status":
		data = map[string]any{"message": "账号连接正常", "nickname": me.Nickname, "points": me.Points}
	case "search":
		step("读取资源列表")
		raw, e := c.do("GET", "/resource-detail?key="+dianResourceKey(kind, id), nil)
		if e != nil {
			return pluginapi.Result{}, e
		}
		shares, e := parseDianDetail(raw)
		if e != nil {
			return pluginapi.Result{}, e
		}
		data = resourceResult(shares)
	case "checkin":
		step("提交普通签到")
		if err = pending(); err != nil {
			return pluginapi.Result{}, err
		}
		raw, e := c.do("POST", "/signin", map[string]string{"mode": "normal"})
		if e != nil && !dianAlreadySigned(e) {
			return pluginapi.Result{}, e
		}
		points := -1
		if current, e := c.me(); e == nil {
			points = current.Points
		}
		message := "今日已签到"
		if e == nil {
			message = dianCheckinText(raw, points)
		} else if points >= 0 {
			message += fmt.Sprintf("，当前积分 %d", points)
		}
		data = map[string]any{"message": message, "points": points}
	case "unlock":
		step("提交资源解锁")
		shareID, resourceID, _ := parseDianRef(resourceID)
		body := map[string]int{"share_id": shareID}
		if resourceID > 0 {
			body["resource_id"] = resourceID
		}
		if err = pending(); err != nil {
			return pluginapi.Result{}, err
		}
		raw, e := c.do("POST", "/unlock", body)
		if e != nil {
			return pluginapi.Result{}, e
		}
		out, e := parseDianUnlock(raw)
		if e != nil {
			return pluginapi.Result{}, e
		}
		links := out.Links
		if out.Share != "" {
			links = []string{out.Share}
		}
		data = map[string]any{"message": "资源已解锁", "links": links, "password": out.Code, "already": out.Again, "balance": out.Balance}
	}
	result := pluginapi.Result{Message: fmt.Sprint(data["message"]), Data: data}
	if key != "" {
		b, _ := json.Marshal(result)
		if len(b) < 44<<10 {
			if e := h.Call("kv.set", map[string]any{"key": key, "value": receipt{Until: now.Unix() + ttl, Result: result}}, nil); e != nil {
				return pluginapi.Result{}, e
			}
		}
	}
	if action == "checkin" && r.Config["notify"] == true {
		if e := h.Call("notify", map[string]any{"message": result.Message}, nil); e != nil {
			return pluginapi.Result{}, e
		}
	}
	_ = h.Call("task.progress", pluginapi.Progress{Done: 1, Total: 1, Percent: 100, Message: result.Message}, nil)
	return result, nil
}
func main() {
	inv, e := pluginwasm.Read()
	if e != nil {
		pluginwasm.Return(pluginapi.Result{Message: "插件输入格式错误", Failed: true})
		return
	}
	if inv.Mode == "health" || inv.Mode == "migrate" {
		pluginwasm.Return(pluginapi.Result{Message: "癫影插件已就绪"})
		return
	}
	result, e := execute(wasmHost{}, inv.Request, time.Sleep)
	if e != nil {
		result = pluginapi.Result{Message: e.Error(), Failed: true}
	}
	pluginwasm.Return(result)
}
