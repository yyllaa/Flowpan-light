package pluginapi

import "testing"

func TestPluginAdaptiveStorageErrorContract(t *testing.T) {
	for _, e := range []*StorageError{{Message: "provider failed", Code: "AUTH_EXPIRED", Status: 401}, {Message: "quota", Code: "quota.full"}, {Message: "failed"}} {
		if err := e.Validate(); err != nil || e.HTTPStatus() != e.Status || e.BusinessCode() != e.Code {
			t.Fatal(e, err)
		}
	}
	for _, e := range []*StorageError{nil, {}, {Message: "error", Status: 999}, {Message: "error", Code: "token=private value"}} {
		if e.Validate() == nil {
			t.Fatal("invalid diagnostic accepted")
		}
	}
}
