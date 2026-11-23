package pluginwasm

import "flowpan/app/pluginapi"

// StoragePaths never accepts an account from plugin input. References are scoped
// to the current host-created storage invocation and persist through restarts.
func StoragePaths(input pluginapi.StoragePathsRequest) ([]pluginapi.StoragePathRef, error) {
	var refs []pluginapi.StoragePathRef
	err := Call("storage.paths", input, &refs)
	return refs, err
}
