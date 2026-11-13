package pluginapi

import (
	"path/filepath"
	"strings"
)

// IntersectScope freezes submission-time grants and applies current revocations.
// New grants cannot widen a task that was already queued.
func IntersectScope(frozen, current Scope) Scope {
	out := Scope{DisabledTools: frozen.DisabledTools}
	for _, a := range frozen.Accounts {
		for _, b := range current.Accounts {
			if strings.TrimPrefix(a, "pool:") == strings.TrimPrefix(b, "pool:") {
				out.Accounts = append(out.Accounts, strings.TrimPrefix(a, "pool:"))
				break
			}
		}
	}
	normal := func(p string) string { return strings.TrimSuffix(filepath.ToSlash(filepath.Clean(p)), "/") }
	within := func(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
	seen := map[string]bool{}
	for _, a := range frozen.Directories {
		for _, b := range current.Directories {
			x, y := normal(a), normal(b)
			value := ""
			if within(x, y) {
				value = a
			} else if within(y, x) {
				value = b
			}
			if value != "" && !seen[normal(value)] {
				seen[normal(value)] = true
				out.Directories = append(out.Directories, value)
			}
		}
	}
	if frozen.AccountDirectories != nil || current.AccountDirectories != nil {
		out.AccountDirectories = make(map[string][]string)
		out.Directories = nil
		for _, account := range out.Accounts {
			a := Scope{Accounts: []string{account}, Directories: frozen.DirectoryRoots(account)}
			b := Scope{Accounts: []string{account}, Directories: current.DirectoryRoots(account)}
			out.AccountDirectories[account] = IntersectScope(a, b).Directories
		}
	}
	if frozen.AccountDirectoryIDs != nil && current.AccountDirectoryIDs != nil {
		out.AccountDirectoryIDs = map[string][]string{}
		for _, account := range out.Accounts {
			for _, id := range frozen.AccountDirectoryIDs[account] {
				if current.AllowsDirectoryID(account, id) {
					out.AccountDirectoryIDs[account] = append(out.AccountDirectoryIDs[account], id)
				}
			}
		}
	}
	return out
}
