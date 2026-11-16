package pluginwasm

import "flowpan/app/pluginapi"

func GenerateED2K(account, id string) (pluginapi.ED2KTask, error) {
	var result pluginapi.ED2KTask
	err := Call("ed2k.submit", map[string]any{"account": account, "id": id}, &result)
	return result, err
}
func ListED2K(query pluginapi.ED2KQuery) (pluginapi.ED2KPage, error) {
	var result pluginapi.ED2KPage
	err := Call("ed2k.list", query, &result)
	return result, err
}
func ED2KTask(id string) (pluginapi.ED2KTask, error)   { return ed2kTask("ed2k.task", id) }
func RetryED2K(id string) (pluginapi.ED2KTask, error)  { return ed2kTask("ed2k.retry", id) }
func CancelED2K(id string) (pluginapi.ED2KTask, error) { return ed2kTask("ed2k.cancel", id) }
func RemoveED2K(id string) error                       { return Call("ed2k.remove", map[string]any{"id": id}, nil) }
func ed2kTask(method, id string) (pluginapi.ED2KTask, error) {
	var result pluginapi.ED2KTask
	err := Call(method, map[string]any{"id": id}, &result)
	return result, err
}
