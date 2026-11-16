package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"flowpan/app/pluginapi"
)

// This consumes the public library contract; the generator plugin's private
// files, package identity and installation state are deliberately not involved.
func ed2kOptions(h host, input map[string]any) (pluginapi.Result, error) {
	query := pluginapi.ED2KQuery{Status: "success", Limit: 20}
	b, err := json.Marshal(input)
	if err != nil {
		return pluginapi.Result{}, err
	}
	if err = json.Unmarshal(b, &query); err != nil {
		return pluginapi.Result{}, err
	}
	query.Status = "success"
	var page pluginapi.ED2KPage
	if err = h.Call("ed2k.list", query, &page); err != nil {
		return pluginapi.Result{}, err
	}
	options := []pluginapi.Option{}
	for _, item := range page.Items {
		if item.Status == "success" && item.Link != "" {
			options = append(options, pluginapi.Option{Value: item.ID, Label: fmt.Sprintf("%s · %.2f MiB · %s", item.Name, float64(item.Size)/(1024*1024), item.Path)})
		}
	}
	return pluginapi.Result{Data: map[string]any{"options": options, "total": page.Total, "offset": page.Offset, "limit": page.Limit}}, nil
}

func attachED2K(h host, input map[string]any) error {
	ids, err := inputLines(input["ed2k_ids"], "ED2K 记录", 500)
	if err != nil {
		return err
	}
	delete(input, "ed2k_ids")
	if len(ids) == 0 {
		return nil
	}
	if input["share_kind"] != "offline" {
		return fmt.Errorf("ED2K 记录仅用于离线资源发布")
	}
	links, err := inputLines(input["offline_url"], "离线链接", 500)
	if err != nil {
		return err
	}
	for _, id := range ids {
		value, err := hex.DecodeString(id)
		if err != nil || len(value) != 20 {
			return fmt.Errorf("ED2K 记录编号无效")
		}
		var item pluginapi.ED2KTask
		if err = h.Call("ed2k.task", map[string]any{"id": id}, &item); err != nil {
			return err
		}
		if item.ID != id || item.Status != "success" || !strings.HasPrefix(item.Link, "ed2k://|file|") {
			return fmt.Errorf("所选 ED2K 记录尚未生成，请重新选择")
		}
		links = append(links, item.Link)
	}
	if len(links) > 500 {
		return fmt.Errorf("离线链接最多 500 条")
	}
	input["offline_url"] = strings.Join(links, "\n")
	return nil
}
