package pluginwasm

import "flowpan/app/pluginapi"

// Emby calls share the host's saved instance connection, proxy and authorization.
// List results retain Emby's public field names; items.get also returns Version.
func EmbyInstances() ([]pluginapi.EmbyInstance, error) {
	var v []pluginapi.EmbyInstance
	err := Call("emby.instances.list", nil, &v)
	return v, err
}
func EmbyCapabilities(instance string, result any) error {
	return Call("emby.capabilities", pluginapi.EmbyInput{InstanceID: instance}, result)
}
func EmbyUsers(instance string, result any) error {
	return Call("emby.users.list", pluginapi.EmbyInput{InstanceID: instance}, result)
}
func EmbyLibraries(instance, user string, result any) error {
	return Call("emby.libraries.list", pluginapi.EmbyInput{InstanceID: instance, UserID: user}, result)
}
func EmbyItems(instance, user string, query map[string]string, result any) error {
	return Call("emby.items.list", pluginapi.EmbyInput{InstanceID: instance, UserID: user, Query: query}, result)
}
func EmbyGetItem(instance, id, user string) (pluginapi.EmbyItem, error) {
	var v pluginapi.EmbyItem
	err := Call("emby.items.get", pluginapi.EmbyInput{InstanceID: instance, ID: id, UserID: user}, &v)
	return v, err
}
func EmbyPersons(instance string, query map[string]string, result any) error {
	return Call("emby.persons.list", pluginapi.EmbyInput{InstanceID: instance, Query: query}, result)
}
func EmbyGetPerson(instance, name string) (pluginapi.EmbyItem, error) {
	var v pluginapi.EmbyItem
	err := Call("emby.persons.get", pluginapi.EmbyInput{InstanceID: instance, Name: name}, &v)
	return v, err
}
func EmbyPersonCredits(instance, id string, query map[string]string, result any) error {
	return Call("emby.persons.credits", pluginapi.EmbyInput{InstanceID: instance, ID: id, Query: query}, result)
}
func EmbySessions(instance string, query map[string]string, result any) error {
	return Call("emby.sessions.list", pluginapi.EmbyInput{InstanceID: instance, Query: query}, result)
}
func EmbyImages(instance, id string, result any) error {
	return Call("emby.images.list", pluginapi.EmbyInput{InstanceID: instance, ID: id}, result)
}
func EmbyDownloadImage(p pluginapi.EmbyInput) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("emby.images.download", p, &v)
	return v, err
}
func EmbyWrite(operation string, p pluginapi.EmbyInput) (pluginapi.EmbyReceipt, error) {
	var v pluginapi.EmbyReceipt
	err := Call(operation, p, &v)
	return v, err
}
func EmbyUpdateMetadata(p pluginapi.EmbyInput) (pluginapi.EmbyReceipt, error) {
	return EmbyWrite("emby.metadata.update", p)
}
func EmbyUploadImage(p pluginapi.EmbyInput) (pluginapi.EmbyReceipt, error) {
	return EmbyWrite("emby.images.upload", p)
}
func EmbyRefreshItem(p pluginapi.EmbyInput) (pluginapi.EmbyReceipt, error) {
	return EmbyWrite("emby.items.refresh", p)
}
func EmbyScanLibrary(instance, key string) (pluginapi.EmbyReceipt, error) {
	return EmbyWrite("emby.library.scan", pluginapi.EmbyInput{InstanceID: instance, Key: key})
}
func EmbyNotifyMedia(p pluginapi.EmbyInput) (pluginapi.EmbyReceipt, error) {
	return EmbyWrite("emby.media.updated", p)
}
func EmbyTasks(instance string, result any) error {
	return Call("emby.tasks.list", pluginapi.EmbyInput{InstanceID: instance}, result)
}
func EmbyOperation(instance, operation, key string) (pluginapi.EmbyReceipt, error) {
	var v pluginapi.EmbyReceipt
	err := Call("emby.operations.get", pluginapi.EmbyInput{InstanceID: instance, Operation: operation, Key: key}, &v)
	return v, err
}
