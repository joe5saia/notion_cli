package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/notion"
)

func TestEmitPollInclusiveLowerBound(t *testing.T) {
	t.Parallel()

	since := time.Date(2024, 4, 10, 15, 30, 0, 0, time.UTC)
	until := since.Add(2 * time.Minute)

	pages := []notion.Page{
		{ID: "page-1"},
		{ID: "page-2"},
	}

	client := &recordingChangeClient{
		t:                  t,
		expectedKeys:       []string{"on_or_after"},
		perCallPages:       [][]notion.Page{pages},
		expectedDataSource: "ds-1",
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	opts := &syncWatchOptions{dataSourceID: "ds-1"}
	if err := opts.emitPoll(context.Background(), client, enc, since, until, false); err != nil {
		t.Fatalf("emitPoll failed: %v", err)
	}

	var output watchOutput
	if err := json.NewDecoder(&buf).Decode(&output); err != nil {
		t.Fatalf("decode poll output: %v", err)
	}

	if output.Kind != "poll" {
		t.Fatalf("expected kind poll, got %q", output.Kind)
	}
	if output.Count != len(pages) {
		t.Fatalf("expected count %d, got %d", len(pages), output.Count)
	}
	if output.Window == nil {
		t.Fatal("expected window metadata")
	}
	if !output.Window.Since.Equal(since) {
		t.Fatalf("expected window since %s, got %s", since, output.Window.Since)
	}
	if !output.Window.Until.Equal(until) {
		t.Fatalf("expected window until %s, got %s", until, output.Window.Until)
	}
}

func TestEmitPollExclusiveLowerBound(t *testing.T) {
	t.Parallel()

	since := time.Date(2024, 7, 5, 10, 0, 0, 0, time.UTC)
	until := since.Add(30 * time.Second)

	client := &recordingChangeClient{
		t:                  t,
		expectedKeys:       []string{"after"},
		perCallPages:       [][]notion.Page{{}},
		expectedDataSource: "ds-1",
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	opts := &syncWatchOptions{dataSourceID: "ds-1"}
	if err := opts.emitPoll(context.Background(), client, enc, since, until, true); err != nil {
		t.Fatalf("emitPoll failed: %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected 1 query, got %d", client.calls)
	}
}

func TestWatchRuntimeUsesExclusiveLowerBoundAfterBootstrap(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 8, 20, 12, 0, 0, 0, time.UTC)

	opts := &syncWatchOptions{
		dataSourceID: "ds-1",
		pollInterval: time.Second,
		lookback:     time.Minute,
		initialSince: now.Add(-5 * time.Minute),
	}
	opts.setDisableWebhook(true)

	client := &recordingChangeClient{
		t:                  t,
		expectedKeys:       []string{"on_or_after", "after"},
		perCallPages:       [][]notion.Page{{}, {}},
		expectedDataSource: "ds-1",
	}

	cmd := &cobra.Command{Use: "watch"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	rt := newWatchRuntime(cmd, opts, client)

	if err := rt.bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if err := rt.pollNext(context.Background()); err != nil {
		t.Fatalf("pollNext failed: %v", err)
	}

	if client.calls != len(client.expectedKeys) {
		t.Fatalf("expected %d queries, got %d", len(client.expectedKeys), client.calls)
	}
}

type recordingChangeClient struct {
	t                  testing.TB
	expectedKeys       []string
	perCallPages       [][]notion.Page
	expectedDataSource string
	calls              int
}

func (c *recordingChangeClient) QueryDataSource(
	_ context.Context,
	dataSourceID string,
	req notion.QueryDataSourceRequest,
) (notion.QueryDataSourceResponse, error) {
	c.t.Helper()

	if c.expectedDataSource != "" && dataSourceID != c.expectedDataSource {
		c.t.Fatalf("unexpected data source ID: want %s, got %s", c.expectedDataSource, dataSourceID)
	}

	if c.calls >= len(c.expectedKeys) {
		c.t.Fatalf("unexpected extra query, filter: %#v", req.Filter)
	}

	lowerKey := resolveLowerBoundKey(c.t, req.Filter)
	if lowerKey != c.expectedKeys[c.calls] {
		c.t.Fatalf("unexpected lower bound key: want %s, got %s", c.expectedKeys[c.calls], lowerKey)
	}

	var pages []notion.Page
	if c.calls < len(c.perCallPages) {
		pages = c.perCallPages[c.calls]
	}

	c.calls++

	return notion.QueryDataSourceResponse{
		Results: pages,
		HasMore: false,
	}, nil
}

func resolveLowerBoundKey(t testing.TB, filter any) string {
	t.Helper()

	filterMap, ok := filter.(map[string]any)
	if !ok {
		t.Fatalf("filter is not map[string]any: %#v", filter)
	}
	rawWindow, ok := filterMap["last_edited_time"]
	if !ok {
		t.Fatalf("filter missing last_edited_time field: %#v", filterMap)
	}
	window, ok := rawWindow.(map[string]any)
	if !ok {
		t.Fatalf("window is not map[string]any: %#v", rawWindow)
	}
	for key := range window {
		if key == "on_or_before" {
			continue
		}
		return key
	}
	t.Fatalf("could not resolve lower bound key: %#v", window)
	return ""
}
