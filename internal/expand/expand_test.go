package expand_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/yourorg/notionctl/internal/expand"
	"github.com/yourorg/notionctl/internal/notion"
)

type stubFetcher struct {
	pages    map[string]notion.Page
	requests []string
	mu       sync.Mutex
}

func (s *stubFetcher) RetrievePage(_ context.Context, id string) (notion.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, id)
	page, ok := s.pages[id]
	if !ok {
		return notion.Page{}, fmt.Errorf("missing page %s", id)
	}
	return page, nil
}

func TestFirstLevel(t *testing.T) {
	client := &stubFetcher{
		pages: map[string]notion.Page{
			"rel-1": {ID: "rel-1", Properties: map[string]notion.PropertyValue{}},
		},
	}

	pages := []notion.Page{
		{
			ID: "page-1",
			Properties: map[string]notion.PropertyValue{
				"Assignee": {
					Type:     "relation",
					Relation: []notion.RelationReference{{ID: "rel-1"}},
				},
			},
		},
		{
			ID: "page-2",
			Properties: map[string]notion.PropertyValue{
				"Assignee": {
					Type:     "relation",
					Relation: []notion.RelationReference{{ID: "rel-1"}},
				},
			},
		},
	}

	refs := []notion.PropertyReference{{ID: "prop-assignee", Name: "Assignee", Type: "relation"}}

	if err := expand.FirstLevel(context.Background(), client, pages, refs); err != nil {
		t.Fatalf("FirstLevel returned error: %v", err)
	}

	if len(client.requests) != 1 || client.requests[0] != "rel-1" {
		t.Fatalf("expected single fetch of rel-1, got %+v", client.requests)
	}

	for i := range pages {
		expanded := pages[i].ExpandedRelations["Assignee"]
		if len(expanded) != 1 || expanded[0].ID != "rel-1" {
			t.Fatalf("expected relation on page %d to be expanded, got %#v", i, expanded)
		}
	}
}
