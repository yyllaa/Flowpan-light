package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestDirectoryScopeAccountPairsAndRevocation(t *testing.T) {
	frozen := Scope{Accounts: []string{"pool:main", "src_a", "src_b"}, AccountDirectories: map[string][]string{
		"main": {"/115"}, "src_a": {"/a"}, "src_b": {"/b"},
	}}
	current := Scope{Accounts: []string{"main", "src_a", "src_new"}, AccountDirectories: map[string][]string{
		"main": {"/115"}, "src_a": {"/a/movies", "/new"}, "src_new": {"/"},
	}}
	frozen.AccountDirectoryIDs = map[string][]string{"src_a": {"old", "common"}, "src_b": {"common"}}
	current.AccountDirectoryIDs = map[string][]string{"src_a": {"common", "new"}, "src_new": {"old"}}
	result := IntersectScope(frozen, current)
	if !result.AllowsDirectoryID("src_a", "common") || result.AllowsDirectoryID("src_a", "old") || result.AllowsDirectoryID("src_a", "new") || result.AllowsDirectoryID("src_b", "common") || result.AllowsDirectoryID("src_new", "old") {
		t.Fatal("opaque ID snapshot widened or retained revoked account", result)
	}
	for _, tc := range []struct {
		account, path string
		allowed       bool
	}{
		{"src_a", "/a/movies/film.mkv", true}, {"src_a", "/a/books", false},
		{"src_a", "/115/film.strm", false}, {"main", "/a/movies", false},
		{"src_b", "/b/film", false}, {"src_new", "/film", false},
		{"src_a", "/new/film", false}, {"src_a", "/a/movies2/film", false},
		{"src_a", "/a/movies/../../115/film", false}, {"src_a", `\a\movies`, false},
		{"pool:main", "/115/film", true},
	} {
		if got := result.AllowsDirectory(tc.account, tc.path); got != tc.allowed {
			t.Errorf("%s %q allowed=%v want=%v", tc.account, tc.path, got, tc.allowed)
		}
	}
	// Old queued tasks must also honor newly narrowed account-specific grants.
	legacy := Scope{Accounts: []string{"main", "src_a"}, Directories: []string{"/"}}
	if got := IntersectScope(legacy, current); !got.AllowsDirectory("src_a", "/a/movies/film") || got.AllowsDirectory("src_a", "/115/film") {
		t.Fatal("legacy queue ignored account-specific revocation", got)
	}
	for _, raw := range []string{
		`{"accounts":["main"],"directories":["/"],"account_directories":{}}`,
		`{"accounts":["main"],"directories":["/"],"account_directories":{"main":[]}}`,
	} {
		var scope Scope
		if err := json.Unmarshal([]byte(raw), &scope); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(scope)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &scope); err != nil {
			t.Fatal(err)
		}
		if scope.AllowsDirectory("main", "/115") {
			t.Fatal("empty map fell back to shared roots", string(b))
		}
	}
	t.Log("DIRECTORY_SCOPE_OK cross_account=denied revoked=denied queued_expansion=denied empty_map=denied legacy=compatible")
}

func TestDirectoryScopeRejectsUnboundOrInvalidRoots(t *testing.T) {
	for _, scope := range []Scope{
		{Accounts: []string{"src_a"}, AccountDirectories: map[string][]string{"main": {"/"}}},
		{Accounts: []string{"main"}, AccountDirectories: map[string][]string{"pool:main": {"/"}}},
		{Accounts: []string{"src_a"}, AccountDirectories: map[string][]string{"src_a": {"relative"}}},
		{Accounts: []string{"src_a"}, AccountDirectories: map[string][]string{"src_a": {"/bad\x00name"}}},
	} {
		if scope.ValidateDirectoryScopes() == nil {
			t.Fatal("invalid directory scope accepted", scope)
		}
	}
}
