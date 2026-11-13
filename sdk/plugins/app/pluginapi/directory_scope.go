package pluginapi

import (
	"path/filepath"
	"strings"
)

func ScopeAccount(account string) string { return strings.TrimPrefix(account, "pool:") }

func (s Scope) HasAccount(account string) bool {
	for _, v := range s.Accounts {
		if ScopeAccount(v) == ScopeAccount(account) {
			return true
		}
	}
	return false
}

// A non-nil account map replaces the legacy shared list. A missing account or
// an empty list denies access; it must never fall back to another account's roots.
func (s Scope) DirectoryRoots(account string) []string {
	if !s.HasAccount(account) {
		return nil
	}
	if s.AccountDirectories != nil {
		return s.AccountDirectories[ScopeAccount(account)]
	}
	return s.Directories
}

// Directory IDs are derived by the host from native directory bindings.
func (s Scope) AllowsDirectoryID(account, id string) bool {
	if id == "" || ScopeAccount(account) == "local" || !s.HasAccount(account) {
		return false
	}
	for _, root := range s.AccountDirectoryIDs[ScopeAccount(account)] {
		if id == root {
			return true
		}
	}
	return false
}

func validScopePath(account, value string) bool {
	if value == "" || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	if ScopeAccount(account) == "local" {
		return filepath.IsAbs(value)
	}
	// Cloud paths use slash-separated absolute names on every host OS.
	return strings.HasPrefix(value, "/") && !strings.Contains(value, "\\")
}

func scopePath(value string) string {
	return strings.TrimSuffix(filepath.ToSlash(filepath.Clean(value)), "/")
}

func (s Scope) AllowsDirectory(account, value string) bool {
	if !validScopePath(account, value) {
		return false
	}
	p := scopePath(value)
	for _, root := range s.DirectoryRoots(account) {
		if validScopePath(account, root) {
			r := scopePath(root)
			if p == r || strings.HasPrefix(p, r+"/") {
				return true
			}
		}
	}
	return false
}

// ValidateDirectoryScopes rejects ambiguous aliases and orphaned map entries.
// Existing shared-list grants remain readable until explicitly saved by the UI.
func (s Scope) ValidateDirectoryScopes() error {
	if len(s.AccountDirectories) > 50 || len(s.AccountDirectoryIDs) > 50 {
		return &HostError{Code: "scope_denied", Message: "授权范围过多"}
	}
	count := 0
	for account, roots := range s.AccountDirectories {
		if account != ScopeAccount(account) || !s.HasAccount(account) {
			return &HostError{Code: "scope_denied", Message: "目录未绑定授权账号"}
		}
		count += len(roots)
		for _, root := range roots {
			if !validScopePath(account, root) {
				return &HostError{Code: "scope_denied", Message: "请选择完整的授权目录"}
			}
		}
	}
	for account, ids := range s.AccountDirectoryIDs {
		if account != ScopeAccount(account) || account == "local" || !s.HasAccount(account) {
			return &HostError{Code: "scope_denied", Message: "目录未绑定授权账号"}
		}
		count += len(ids)
		for _, id := range ids {
			if id == "" || strings.ContainsAny(id, "\x00\r\n") {
				return &HostError{Code: "scope_denied", Message: "目录标识无效"}
			}
		}
	}
	if count > 250 {
		return &HostError{Code: "scope_denied", Message: "授权范围过多"}
	}
	return nil
}
