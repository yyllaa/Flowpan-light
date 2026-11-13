package main

import (
	"encoding/json"
	"errors"
	"flowpan/app/pluginapi"
	"fmt"
	"strconv"
	"time"
)

func dianPositiveID(value any, zero bool) (int, bool) {
	s := fmt.Sprint(value)
	if number, ok := value.(float64); ok {
		s = strconv.FormatFloat(number, 'f', -1, 64)
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && (n > 0 || zero && n == 0)
}

func executeOpenAPI(h host, r pluginapi.Request) (pluginapi.Result, error) {
	action := r.ActionID
	mode := "normal"
	if action == "signin-lucky" {
		if !r.Confirmed {
			return pluginapi.Result{}, fmt.Errorf("请先确认幸运签到")
		}
		action, mode = "checkin", "lucky"
	}
	if action != "status" && action != "checkin" && action != "search" && action != "unlock" {
		return executeTool(h, r)
	}
	if r.TaskID == "" {
		return pluginapi.Result{}, fmt.Errorf("缺少后台任务编号")
	}
	kind, _ := r.Input["media_type"].(string)
	id, validID := dianPositiveID(r.Input["tmdb_id"], false)
	query := map[string]any{"tmdb_id": id, "media_type": kind, "page": 1, "size": 50, "sort": "hot"}
	seasonKey := "all"
	if action == "search" {
		if !validID || kind != "movie" && kind != "tv" {
			return pluginapi.Result{}, fmt.Errorf("请选择电影/剧集并填写 TMDB ID")
		}
		if value, specified := r.Input["season"]; specified && value != nil && value != "" {
			season, ok := dianPositiveID(value, true)
			if kind != "tv" || !ok {
				return pluginapi.Result{}, fmt.Errorf("剧集季数无效")
			}
			query["season"] = season
			seasonKey = strconv.Itoa(season)
		}
	}
	ref, _ := r.Input["resource_id"].(string)
	if action == "unlock" {
		if !r.Confirmed {
			return pluginapi.Result{}, fmt.Errorf("请先确认资源解锁")
		}
		if _, _, ok := parseDianRef(ref); !ok {
			return pluginapi.Result{}, fmt.Errorf("资源编号无效")
		}
	}
	client := dianHostClient{h: h}
	var identity pluginapi.SDKResponse
	if err := h.Call("sdk.call", pluginapi.SDKRequest{Provider: "dian115", Operation: "identity"}, &identity); err != nil {
		return pluginapi.Result{}, err
	}
	if identity.Account == "" {
		return pluginapi.Result{}, fmt.Errorf("请配置癫影 OpenAPI Key")
	}
	account := identity.Account
	now := time.Now()
	key, ttl := "", int64(0)
	switch action {
	case "search":
		key, ttl = fmt.Sprintf("%s-search-%s-%d-%s", account, kind, id, seasonKey), 600
	case "checkin":
		key, ttl = account+"-checkin-"+now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("20060102"), 86400
	case "unlock":
		key, ttl = account+"-unlock-"+accountKey(ref)[:16], 86400*30
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
	pending := func() error {
		return h.Call("kv.set", map[string]any{"key": key, "value": receipt{Pending: true}}, nil)
	}
	apiError := func(err error) error {
		var denied *pluginapi.HostError
		var rejected *pluginapi.SDKFailure
		// Only an authenticated 4xx response proves that the write was rejected.
		// Timeouts, broken envelopes and 5xx keep the pending receipt intact.
		if key != "" && writeRejected(err) && (action == "checkin" || action == "unlock") {
			if e := h.Call("kv.set", map[string]any{"key": key, "value": receipt{}}, nil); e != nil {
				return e
			}
		}
		if errors.As(err, &denied) {
			return denied
		}
		if errors.As(err, &rejected) {
			message := rejected.Error()
			if rejected.Status == 429 {
				return fmt.Errorf("OpenAPI 请求达到限额，请稍后手动重试：%s", truncateStr(message, 300))
			}
			return fmt.Errorf("OpenAPI 请求失败：%s", truncateStr(message, 300))
		}
		return fmt.Errorf("OpenAPI 请求失败，请检查网络或运行记录")
	}
	var data map[string]any
	switch action {
	case "status":
		step("通过 OpenAPI 检查连接")
		me, e := client.GetUserInfo()
		if e != nil {
			return pluginapi.Result{}, apiError(e)
		}
		if me.ID <= 0 {
			return pluginapi.Result{}, fmt.Errorf("OpenAPI 账号响应无效")
		}
		data = map[string]any{"message": "账号连接正常", "nickname": me.Nickname, "points": me.Points, "connection": "OpenAPI V2", "entitlements": me.Entitlements, "user": me.Profile}
	case "search":
		step("通过 OpenAPI 读取资源列表")
		shares := []dianShare{}
		seen := map[int]bool{}
		for page := 1; page <= 4; page++ {
			query["page"] = page
			var raw json.RawMessage
			if e := client.ListShares(query, &raw); e != nil {
				return pluginapi.Result{}, apiError(e)
			}
			items, count, e := parseDianOpenAPIList(raw, kind)
			if e != nil {
				return pluginapi.Result{}, e
			}
			added := 0
			for _, item := range items {
				if !seen[item.ID] {
					shares = append(shares, item)
					seen[item.ID] = true
					added++
				}
			}
			if count < 50 || added == 0 {
				break
			}
		}
		data = resourceResult(shares)
	case "checkin":
		label := "普通签到"
		if mode == "lucky" {
			label = "幸运签到"
		}
		step("通过 OpenAPI 提交" + label)
		if err := pending(); err != nil {
			return pluginapi.Result{}, err
		}
		var raw json.RawMessage
		e := client.Signin(mode, &raw)
		if e != nil && !dianAlreadySigned(e) {
			return pluginapi.Result{}, apiError(e)
		}
		points := -1
		var signed struct {
			Balance *int `json:"new_balance"`
			Points  *int `json:"points"`
		}
		_ = json.Unmarshal(raw, &signed)
		if signed.Balance != nil {
			points = *signed.Balance
		} else if signed.Points != nil {
			points = *signed.Points
		}
		if points < 0 {
			if current, e := client.GetUserInfo(); e == nil {
				points = current.Points
			}
		}
		message := "今日已签到"
		if e == nil {
			message = dianCheckinText(raw, points)
		} else if points >= 0 {
			message += fmt.Sprintf("，当前积分 %d", points)
		}
		data = map[string]any{"message": message, "points": points}
	case "unlock":
		step("通过 OpenAPI 提交资源解锁")
		shareID, resourceID, _ := parseDianRef(ref)
		body := map[string]any{"share_id": shareID}
		if resourceID > 0 {
			body["resource_id"] = resourceID
		}
		if err := pending(); err != nil {
			return pluginapi.Result{}, err
		}
		var raw json.RawMessage
		if e := client.Unlock(body, &raw); e != nil {
			return pluginapi.Result{}, apiError(e)
		}
		unlocked, e := parseDianUnlock(raw)
		if e != nil {
			return pluginapi.Result{}, e
		}
		links := unlocked.Links
		if unlocked.Share != "" {
			links = []string{unlocked.Share}
		}
		data = map[string]any{"message": "资源已解锁", "links": links, "password": unlocked.Code, "already": unlocked.Again, "balance": unlocked.Balance}
	}
	data["action_id"] = r.ActionID
	result := pluginapi.Result{Message: fmt.Sprint(data["message"]), Data: data}
	if key != "" {
		raw, _ := json.Marshal(result)
		if len(raw) < 44<<10 {
			if err := h.Call("kv.set", map[string]any{"key": key, "value": receipt{Until: now.Unix() + ttl, Result: result}}, nil); err != nil {
				return pluginapi.Result{}, err
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

func parseDianOpenAPIList(raw []byte, mediaType string) ([]dianShare, int, error) {
	var result struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.Items == nil {
		return nil, 0, fmt.Errorf("解析癫影 OpenAPI 资源列表失败")
	}
	items := []dianShare{}
	for _, raw := range result.Items {
		var item dianShare
		var ids struct {
			ShareID     int    `json:"share_id"`
			EpisodesCSV string `json:"episodes_csv"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, 0, fmt.Errorf("解析癫影 OpenAPI 分享失败")
		}
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, 0, fmt.Errorf("解析癫影 OpenAPI 分享编号失败")
		}
		if ids.ShareID > 0 {
			item.ID = ids.ShareID
		}
		if item.ID <= 0 || item.Status != "" && item.Status != "active" {
			continue
		}
		if item.MediaType == "" {
			item.MediaType = mediaType
		}
		if item.Episodes == "" {
			item.Episodes = ids.EpisodesCSV
		}
		items = append(items, item)
	}
	return items, len(result.Items), nil
}
