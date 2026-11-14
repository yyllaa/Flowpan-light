package pluginwasm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flowpan/app/pluginapi"
)

func CreateArtifact(p pluginapi.ArtifactCreate) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("artifacts.create", p, &v)
	return v, err
}
func WriteArtifact(id string, offset int64, b []byte) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("artifacts.write", map[string]any{"id": id, "offset": offset, "base64": base64.StdEncoding.EncodeToString(b)}, &v)
	return v, err
}
func SealArtifact(id string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("artifacts.seal", map[string]any{"id": id}, &v)
	return v, err
}
func StatArtifact(id string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("artifacts.stat", map[string]any{"id": id}, &v)
	return v, err
}
func ReadArtifact(id string, offset int64, limit int) (pluginapi.MediaChunk, error) {
	var v pluginapi.MediaChunk
	err := Call("artifacts.read", map[string]any{"id": id, "offset": offset, "limit": limit}, &v)
	return v, err
}
func ReleaseArtifact(id string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("artifacts.release", map[string]any{"id": id}, &v)
	return v, err
}

// PutArtifact resumes a bounded in-memory generated object in 32 KiB segments.
func PutArtifact(key, name string, b []byte) (pluginapi.Artifact, error) {
	h := sha256.Sum256(b)
	v, err := CreateArtifact(pluginapi.ArtifactCreate{Key: key, Name: name, Size: int64(len(b)), SHA256: hex.EncodeToString(h[:])})
	if err != nil || v.Status == "sealed" {
		return v, err
	}
	for offset := v.Written; offset < int64(len(b)); {
		end := min(offset+32<<10, int64(len(b)))
		v, err = WriteArtifact(v.ID, offset, b[offset:end])
		if err != nil {
			return v, err
		}
		offset = v.Written
	}
	return SealArtifact(v.ID)
}
func ReadFile(account, id string, offset int64, limit int) (pluginapi.MediaChunk, error) {
	var v pluginapi.MediaChunk
	err := Call("files.read", map[string]any{"account": account, "id": id, "offset": offset, "limit": limit}, &v)
	return v, err
}
func MutateFiles(operation string, p pluginapi.FileMutation, result any) error {
	return Call("files."+operation, p, result)
}
func IdentifyMedia(name string, ai, tmdb bool, result any) error {
	return Call("media.identify", map[string]any{"name": name, "ai": ai, "tmdb": tmdb}, result)
}
func AIStatus(result any) error { return Call("ai.status", nil, result) }
func CompleteAI(p pluginapi.AIComplete) (pluginapi.AICompletion, error) {
	var v pluginapi.AICompletion
	err := Call("ai.complete", p, &v)
	return v, err
}
func ServiceUpload(p pluginapi.ServiceUpload, result any) error {
	return Call("service.upload", p, result)
}
func CreateCAS(key, fileTask, name string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("cas.create", map[string]any{"key": key, "file_task": fileTask, "name": name}, &v)
	return v, err
}
func InspectCAS(text string, result any) error {
	return Call("cas.inspect", map[string]any{"text": text}, result)
}
func StrmTasks(result any) error { return Call("strm.tasks", nil, result) }
func CreateSTRM(key, name, url string) (pluginapi.Artifact, error) {
	var v pluginapi.Artifact
	err := Call("strm.create", map[string]any{"key": key, "name": name, "url": url}, &v)
	return v, err
}
func InspectSTRM(account, id string, result any) error {
	return Call("strm.inspect", map[string]any{"account": account, "id": id}, result)
}
func PreviewOrganize(p pluginapi.OrganizePreview) (pluginapi.OrganizePreviewResult, error) {
	var v pluginapi.OrganizePreviewResult
	err := Call("organize.preview", p, &v)
	return v, err
}
func ApplyOrganize(id, key string, result any) error {
	return Call("organize.apply", map[string]any{"id": id, "key": key}, result)
}
func RunStrm(id, key string, result any) error {
	return Call("strm.run", map[string]any{"id": id, "key": key}, result)
}
func StrmStatus(id string, result any) error {
	return Call("strm.status", map[string]any{"id": id}, result)
}
func StopOrganize(account, key string, result any) error {
	return Call("organize.stop", map[string]any{"account": account, "key": key}, result)
}
func DependencyStatus(result any) error { return Call("dependencies.status", nil, result) }
func EnsureDependency(name string, result any) error {
	return Call("dependencies.ensure", map[string]any{"name": name}, result)
}
func Downloaders(result any) error { return Call("downloads.list", nil, result) }
func DownloadStatus(id, filter string, page, limit int, result any) error {
	return Call("downloads.status", map[string]any{"id": id, "filter": filter, "page": page, "limit": limit}, result)
}
func AddDownload(id, key, urls string, result any) error {
	return Call("downloads.add", map[string]any{"id": id, "key": key, "urls": urls}, result)
}
func DownloadAction(id, key, action string, hashes []string, result any) error {
	return Call("downloads.action", map[string]any{"id": id, "key": key, "action": action, "hashes": hashes}, result)
}
func ShareOutput(key string, input pluginapi.HostInput, expiresSeconds int, result any) error {
	return Call("shares.create", map[string]any{"key": key, "input": input, "expires_seconds": expiresSeconds}, result)
}
func RevokeShare(id string, result any) error {
	return Call("shares.revoke", map[string]any{"id": id}, result)
}
