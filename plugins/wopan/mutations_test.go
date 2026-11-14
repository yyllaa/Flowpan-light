package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
	"flowpan/plugins/storage/wopan/internal/wopanfixture"
)

const batchPath = "/wohome/pass/file/batch/createTask"

func mutationRef(id, parent string, dir bool, ancestors ...string) string {
	return encodeRef(objectRef{ID: id, Parent: parent, Dir: dir, Ancestors: ancestors})
}
func mutationCall(t *testing.T, s *wopanfixture.Server, ctx context.Context, op string, ids []string, parent string) pluginapi.Result {
	t.Helper()
	raw, _ := json.Marshal(credentials{Access: wopanfixture.TokenA, Refresh: "refresh-a", UID: "uid-a"})
	res, err := execute(ctx, &pluginapi.StorageCall{Account: "fixture", Operation: op, Credentials: raw, Params: pluginapi.StorageParams{IDs: ids, ParentID: parent}}, s.Client())
	if err != nil {
		t.Fatal(err)
	}
	return res
}
func TestWopanMoveFileVersionAndDuplicateSelection(t *testing.T) {
	s := wopanfixture.New()
	defer s.Close()
	file := mutationRef("media-a", "0", false, "0")
	dir := mutationRef("dir-a", "0", true, "0")
	res := mutationCall(t, s, context.Background(), "move", []string{file, file}, dir)
	if res.Failed || s.Count(batchPath) != 1 {
		t.Fatalf("move failed: %+v", res)
	}
	if len(s.Files["a:0"]) != 1 || len(s.Files["a:dir-a"]) != 2 || len(s.Files["b:0"]) != 2 {
		t.Fatal("move/account isolation mismatch")
	}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
}
func TestWopanDeleteFileAndRecursiveDirectory(t *testing.T) {
	s := wopanfixture.New()
	defer s.Close()
	res := mutationCall(t, s, context.Background(), "delete", []string{mutationRef("media-a", "0", false, "0"), mutationRef("dir-a", "0", true, "0")}, "")
	if res.Failed || len(s.Files["a:0"]) != 0 || s.Count(batchPath) != 3 || len(s.Files["b:0"]) != 2 {
		t.Fatalf("delete failed: %+v", res)
	}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
}
func TestWopanMoveDirectoryAndSameParentNoop(t *testing.T) {
	s := wopanfixture.New()
	defer s.Close()
	s.Files["a:0"] = append(s.Files["a:0"], wopanfixture.File{ID: "target", Name: "目标", Type: 0})
	dir := mutationRef("dir-a", "0", true, "0")
	target := mutationRef("target", "0", true, "0")
	for _, c := range []struct {
		ids    []string
		target string
		count  int
	}{
		{[]string{dir}, rootID, 0}, {[]string{dir}, target, 1},
	} {
		res := mutationCall(t, s, context.Background(), "move", c.ids, c.target)
		if res.Failed || s.Count(batchPath) != c.count {
			t.Fatalf("directory move failed: %+v", res)
		}
	}
	if len(s.Files["a:dir-a"]) != 1 || len(s.Files["a:target"]) != 1 {
		t.Fatal("directory children lost")
	}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
}
func TestWopanMutationSelectionValidation(t *testing.T) {
	file := mutationRef("media-a", "0", false, "0")
	dir := mutationRef("dir-a", "0", true, "0")
	for _, c := range []struct {
		name, op string
		ids      []string
		target   string
	}{
		{"empty", "delete", nil, ""}, {"root", "delete", []string{rootID}, ""},
		{"whole-batch", "delete", []string{file, rootID}, ""},
		{"invalid", "delete", []string{"broken"}, ""},
		{"stale", "delete", []string{file, mutationRef("gone", "0", false, "0")}, ""},
		{"wrong-type", "delete", []string{mutationRef("media-a", "0", true, "0")}, ""},
		{"self", "move", []string{dir}, dir},
		{"descendant", "move", []string{dir}, mutationRef("child", "dir-a", true, "0", "dir-a")},
		{"file-target", "move", []string{dir}, file},
		{"stale-target", "move", []string{file}, mutationRef("gone", "0", true, "0")},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := wopanfixture.New()
			defer s.Close()
			res := mutationCall(t, s, context.Background(), c.op, c.ids, c.target)
			if !res.Failed || s.Count(batchPath) != 0 {
				t.Fatalf("invalid selection wrote: %+v", res)
			}
		})
	}
}
func TestWopanMutationErrorsDoNotReplayUnknownWrites(t *testing.T) {
	for _, c := range []struct{ name, code, reply string }{
		{"system-error", "9999", "{}"}, {"refused", "", `{"success":false,"failMessage":"denied"}`},
		{"unknown", "", "{}"}, {"invalid", "", `"bad-receipt"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := wopanfixture.New()
			defer s.Close()
			s.SetBatchReply(c.code, c.reply)
			res := mutationCall(t, s, context.Background(), "delete", []string{mutationRef("media-a", "0", false, "0")}, "")
			if !res.Failed || s.Count(batchPath) != 1 || len(s.Files["a:0"]) != 2 || strings.Contains(res.Message, wopanfixture.TokenA) {
				t.Fatalf("write error lost/replayed: %+v requests=%d", res, s.Count(batchPath))
			}
		})
	}
}
func TestWopanMutationCancellation(t *testing.T) {
	s := wopanfixture.New()
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := mutationCall(t, s, ctx, "delete", []string{mutationRef("media-a", "0", false, "0")}, "")
	if !res.Failed || s.Count(batchPath) != 0 {
		t.Fatal("cancelled mutation sent")
	}
}

func TestWopanMoveMixedSelection(t *testing.T) {
	s := wopanfixture.New()
	defer s.Close()
	s.Files["a:0"] = append(s.Files["a:0"], wopanfixture.File{ID: "target", Name: "目标", Type: 0})
	res := mutationCall(t, s, context.Background(), "move", []string{mutationRef("media-a", "0", false, "0"), mutationRef("dir-a", "0", true, "0")}, mutationRef("target", "0", true, "0"))
	if res.Failed || s.Count(batchPath) != 1 || len(s.Files["a:target"]) != 2 {
		t.Fatalf("mixed move failed: %+v", res)
	}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
}
