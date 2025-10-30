package cmd

import (
	"testing"

	"github.com/yourorg/notionctl/internal/notion"
)

func TestMergeRelationArray(t *testing.T) {
	existing := notion.PropertyValue{
		Type: "relation",
		Relation: []notion.RelationReference{
			{ID: "rel-1"},
		},
	}

	updates := []any{
		map[string]any{"id": "rel-2"},
	}

	merged, err := mergeRelationArray(existing, updates, false)
	if err != nil {
		t.Fatalf("mergeRelationArray returned error: %v", err)
	}
	if len(merged) != 2 || merged[0]["id"] != "rel-1" || merged[1]["id"] != "rel-2" {
		t.Fatalf("unexpected merge result: %#v", merged)
	}

	replaced, err := mergeRelationArray(existing, updates, true)
	if err != nil {
		t.Fatalf("replace merge returned error: %v", err)
	}
	if len(replaced) != 1 || replaced[0]["id"] != "rel-2" {
		t.Fatalf("unexpected replace result: %#v", replaced)
	}
}

func TestNormalizeRelationArrayErrors(t *testing.T) {
	if _, err := normalizeRelationArray([]any{"bad"}); err == nil {
		t.Fatalf("expected error for invalid relation entry")
	}
	if _, err := normalizeRelationArray([]any{map[string]any{"name": "missing"}}); err == nil {
		t.Fatalf("expected error for missing relation id")
	}
}
