package pluginwasm

import "flowpan/app/pluginapi"

func Capabilities() (pluginapi.HostCapabilities, error) {
	var result pluginapi.HostCapabilities
	err := Call("host.capabilities", nil, &result)
	return result, err
}

// ReadSecret is available only with secrets.read and runtime_access on the field.
// The host restricts it to this plugin and run mode; never log or persist its value.
func ReadSecret(key string) (string, error) {
	var result struct {
		Value string `json:"value"`
	}
	err := Call("secrets.read", map[string]any{"key": key}, &result)
	return result.Value, err
}

func Get(key string, result any) error { return Call("kv.get", map[string]any{"key": key}, result) }
func Set(key string, value any) error {
	return Call("kv.set", map[string]any{"key": key, "value": value}, nil)
}
func ListState(result any) error                       { return Call("kv.list", nil, result) }
func Log(message string) error                         { return Call("log", map[string]any{"message": message}, nil) }
func Notify(message string) error                      { return Call("notify", map[string]any{"message": message}, nil) }
func ReportProgress(progress pluginapi.Progress) error { return Call("task.progress", progress, nil) }
func TaskStatus(id string, result any) error {
	return Call("task.status", map[string]any{"id": id}, result)
}
func Accounts(result any) error { return Call("accounts.list", nil, result) }

type ListOptions struct {
	Account string `json:"account"`
	ID      string `json:"id,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
}

func ListFiles(options ListOptions, result any) error { return Call("files.list", options, result) }
func StatFile(account, id string, result any) error {
	return Call("files.stat", map[string]any{"account": account, "id": id}, result)
}
func MediaStatus(result any) error { return Call("media.status", nil, result) }
func SearchMedia(query, mediaType string, result any) error {
	return Call("media.search", map[string]any{"query": query, "media_type": mediaType}, result)
}
func ProbeMedia(path string, result any) error {
	return Call("media.probe", map[string]any{"path": path}, result)
}

func SubmitMedia(request pluginapi.MediaProcess) (pluginapi.MediaTask, error) {
	var result pluginapi.MediaTask
	err := Call("media.submit", request, &result)
	return result, err
}

func MediaTask(id string) (pluginapi.MediaTask, error) {
	var result pluginapi.MediaTask
	err := Call("media.task", map[string]any{"id": id}, &result)
	return result, err
}

func CancelMedia(id string) (pluginapi.MediaTask, error) {
	var result pluginapi.MediaTask
	err := Call("media.cancel", map[string]any{"id": id}, &result)
	return result, err
}

func MediaResult(id string, offset int64, limit int) (pluginapi.MediaChunk, error) {
	var result pluginapi.MediaChunk
	err := Call("media.result", map[string]any{"id": id, "offset": offset, "limit": limit}, &result)
	return result, err
}
func ParseTransfer(text string, result any) error {
	return Call("transfer.parse", map[string]any{"text": text}, result)
}

type HTTPRequest struct {
	URL        string            `json:"url"`
	Method     string            `json:"method,omitempty"`
	Body       string            `json:"body,omitempty"`
	AuthSecret string            `json:"auth_secret,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}
type HTTPResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

func Fetch(request HTTPRequest) (HTTPResponse, error) {
	var result HTTPResponse
	err := Call("http.fetch", request, &result)
	return result, err
}
func ServiceCall(method, path string, body any, result any) error {
	return Call("service.call", map[string]any{"method": method, "path": path, "body": body}, result)
}

// SDKCall returns business data from a host SDK bound to this plugin's key.
// SDK source, signing material and user credentials stay in the host process.
func SDKCall(provider, operation string, input map[string]any) (pluginapi.SDKResponse, error) {
	var result pluginapi.SDKResponse
	err := Call("sdk.call", pluginapi.SDKRequest{Provider: provider, Operation: operation, Input: input}, &result)
	if err == nil && result.Failure != nil {
		err = result.Failure
	}
	return result, err
}
