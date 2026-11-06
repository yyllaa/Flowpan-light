//go:build !wasip1

package pluginwasm

func hostCall(request, out []byte) uint32 { return 0 }
