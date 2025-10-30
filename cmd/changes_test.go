package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestBuildChangesFilter(t *testing.T) {
	since := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	until := since.Add(24 * time.Hour)

	filter, err := buildChangesFilter(since, until)
	if err != nil {
		t.Fatalf("buildChangesFilter returned error: %v", err)
	}
	if filter == "" {
		t.Fatalf("expected filter JSON")
	}
	if !strings.Contains(filter, since.Format(time.RFC3339)) || !strings.Contains(filter, until.Format(time.RFC3339)) {
		t.Fatalf("filter missing timestamps: %s", filter)
	}
}
