//go:build wasip1

package pluginwasm

import "unsafe"

//go:wasmimport flowpan host_call
func importedHostCall(p uint32, n uint32, out uint32, capacity uint32) uint32
func hostCall(request, out []byte) uint32 {
	return importedHostCall(uint32(uintptr(unsafe.Pointer(&request[0]))), uint32(len(request)), uint32(uintptr(unsafe.Pointer(&out[0]))), uint32(len(out)))
}
