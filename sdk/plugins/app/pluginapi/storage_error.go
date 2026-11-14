package pluginapi

import "fmt"

// StorageError preserves provider diagnostics across the WASM boundary. Codes
// are short identifiers; response bodies, URLs and credentials are not codes.
type StorageError struct {
	Code    string `json:"code,omitempty"`
	Status  int    `json:"status,omitempty"`
	Message string `json:"message"`
}

func (e *StorageError) Error() string        { return e.Message }
func (e *StorageError) HTTPStatus() int      { return e.Status }
func (e *StorageError) BusinessCode() string { return e.Code }

func (e *StorageError) Validate() error {
	if e == nil || e.Message == "" || len(e.Message) > 4096 || len(e.Code) > 64 || e.Status != 0 && (e.Status < 100 || e.Status > 599) {
		return fmt.Errorf("invalid storage error")
	}
	for _, c := range e.Code {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return fmt.Errorf("invalid storage error code")
		}
	}
	return nil
}
