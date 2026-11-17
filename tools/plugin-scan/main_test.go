package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanSDKWrappersAliasesConstantsAndDynamicCalls(t *testing.T) {
	dir := t.TempDir()
	sdk := filepath.Join(dir, "sdk")
	plugin := filepath.Join(dir, "plugin")
	for _, p := range []string{sdk, plugin} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, source string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(sdk, "sdk.go"), `package pluginwasm
func Call(method string, args any, target any) error { return nil }
func wrapped(method string) { Call(method, nil, nil) }
func Status() { wrapped("ed2k.task") }
func Indirect() { Status() }
`)
	write(filepath.Join(plugin, "main.go"), `package main
import sdk "flowpan/sdk/pluginwasm"
const method = "kv." + "get"
// sdk.Call("fake.comment", nil, nil)
func main() {
 sdk.Indirect()
 sdk.Call(method, nil, nil)
 sdk.Call("future.missing", nil, nil)
 sdk.Call(dynamic(), nil, nil)
}
`)
	known := map[string]bool{"ed2k.task": true, "kv.get": true}
	got, err := scan(plugin, sdk, known)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tools, []string{"ed2k.task", "kv.get"}) || !reflect.DeepEqual(got.UnknownTools, []string{"future.missing"}) || len(got.DynamicCalls) != 1 {
		t.Fatalf("scan=%+v", got)
	}
	write(filepath.Join(plugin, "hidden_test.go"), `package main
func test() { h.Call("unused.test", nil, nil) }`)
	again, err := scan(plugin, sdk, known)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("test-only code changed release calls: %+v %v", again, err)
	}
}
