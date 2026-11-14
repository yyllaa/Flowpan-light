package pluginwasm

import "flowpan/app/pluginapi"

func FileCapabilities(account string) (pluginapi.FileCapabilities, error) {
	var result pluginapi.FileCapabilities
	err := Call("files.capabilities", map[string]any{"account": account}, &result)
	return result, err
}
func StageFile(request pluginapi.FileStage) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.stage", request, &result)
	return result, err
}
func PublishFile(request pluginapi.FilePublish) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.publish", request, &result)
	return result, err
}
func FileTask(id string) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.task", map[string]any{"id": id}, &result)
	return result, err
}
func CancelFile(id string) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.cancel", map[string]any{"id": id}, &result)
	return result, err
}
func ReleaseFile(id string) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.release", map[string]any{"id": id}, &result)
	return result, err
}
func RestoreFile(id, key string) (pluginapi.FileTask, error) {
	var result pluginapi.FileTask
	err := Call("files.restore", map[string]any{"id": id, "key": key}, &result)
	return result, err
}
func ProbeStagedMedia(id string, result any) error {
	return Call("media.probe", map[string]any{"file_task": id}, result)
}
func ReleaseMedia(id string) (pluginapi.MediaTask, error) {
	var result pluginapi.MediaTask
	err := Call("media.release", map[string]any{"id": id}, &result)
	return result, err
}
