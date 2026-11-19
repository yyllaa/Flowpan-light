package main

import (
	"os"
	"strings"
	"testing"
)

func TestLiveAPIMetadataTags(t *testing.T) {
	raw, err := os.ReadFile("testdata/live-84773.json")
	if err != nil {
		t.Fatal(err)
	}
	shares, _, err := parseDianOpenAPIList(raw, "tv")
	if err != nil || len(shares) != 4 {
		t.Fatalf("live items=%d err=%v", len(shares), err)
	}
	items := resourceResult(shares)["resources"].([]resource)
	for i, want := range []string{"S01-S02", "S03E01-E04", "S02E01-E08", "S01E01-E08"} {
		if !strings.Contains(strings.Join(items[i].Tags, " "), want) {
			t.Errorf("STRUCTURED_TAG_MISSING resource=%d want=%s tags=%v", i+1, want, items[i].Tags)
		}
	}
	for _, want := range []string{"24fps", "内封", "MKV"} {
		if !strings.Contains(strings.Join(items[2].Tags, " "), want) {
			t.Errorf("SPEC_TAG_MISSING want=%s tags=%v", want, items[2].Tags)
		}
	}
}
