package pluginwasm

import "flowpan/app/pluginapi"

func InteractionContext(delivery ...string) (pluginapi.InteractionContext, error) {
	var v pluginapi.InteractionContext
	input := map[string]string{}
	if len(delivery) > 0 {
		input["delivery"] = delivery[0]
	}
	err := Call("interaction.context", input, &v)
	return v, err
}
func ReplyInteraction(p pluginapi.InteractionReply) error { return Call("interaction.reply", p, nil) }
func WaitInteraction(p pluginapi.InteractionSession) error {
	return Call("interaction.session.set", p, nil)
}
func EndInteraction(delivery ...string) error {
	input := map[string]string{}
	if len(delivery) > 0 {
		input["delivery"] = delivery[0]
	}
	return Call("interaction.session.end", input, nil)
}
func StageInteractionAttachment(key, id string, delivery ...string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	input := map[string]string{"key": key, "id": id}
	if len(delivery) > 0 {
		input["delivery"] = delivery[0]
	}
	err := Call("interaction.attachment", input, &v)
	return v, err
}
func SendInteractionFile(p pluginapi.InteractionSendFile) error {
	return Call("interaction.send_file", p, nil)
}
func ProbeArtifactMedia(id string, result any) error {
	return Call("media.probe", map[string]string{"artifact": id}, result)
}
