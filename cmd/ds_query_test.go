package cmd

import (
	"testing"

	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/schema"
)

func TestMapPropertyIdentifiers(t *testing.T) {
	idx := schema.NewIndex(notion.DataSource{
		Properties: map[string]notion.PropertyReference{
			"Title":  {ID: "prop-1", Name: "Title", Type: "title"},
			"Status": {ID: "prop-2", Name: "Status", Type: "status"},
		},
	})

	payload := map[string]any{
		"property": "Title",
		"rich_text": map[string]any{
			"equals": "Example",
		},
		"and": []any{
			map[string]any{"property": "Status", "status": map[string]any{"equals": "Done"}},
		},
	}

	mappedValue := mapPropertyIdentifiers(payload, idx)
	mapped, ok := mappedValue.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", mappedValue)
	}
	if mapped["property"] != "prop-1" {
		t.Fatalf("expected property to be mapped to prop-1, got %v", mapped["property"])
	}
	andItems, ok := mapped["and"].([]any)
	if !ok {
		t.Fatalf("expected and clause slice, got %T", mapped["and"])
	}
	firstClause, ok := andItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected clause map, got %T", andItems[0])
	}
	if firstClause["property"] != "prop-2" {
		t.Fatalf("expected nested property to be mapped to prop-2, got %v", firstClause["property"])
	}
}

func TestSummarizeProperty(t *testing.T) {
	titleValue := notion.PropertyValue{
		Type: "title",
		Title: []notion.RichText{
			{PlainText: "Hello"},
			{PlainText: " World"},
		},
	}
	name := summarizeProperty(titleValue)
	if name != "Hello World" {
		t.Fatalf("unexpected title summary: %q", name)
	}

	status := summarizeProperty(notion.PropertyValue{Type: "status", Status: &notion.StatusValue{Name: "Done"}})
	if status != "Done" {
		t.Fatalf("unexpected status summary: %q", status)
	}

	num := summarizeProperty(notion.PropertyValue{Type: "number", Number: floatPtr(42)})
	if num != "42" {
		t.Fatalf("unexpected number summary: %q", num)
	}

	relationValue := notion.PropertyValue{Type: "relation", Relation: []notion.RelationReference{{ID: "a"}, {ID: "b"}}}
	rel := summarizeProperty(relationValue)
	if rel != "a, b" {
		t.Fatalf("unexpected relation summary: %q", rel)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
