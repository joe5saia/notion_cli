package schema_test

import (
	"testing"

	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/schema"
)

func TestIndexLookups(t *testing.T) {
	ds := notion.DataSource{
		Properties: map[string]notion.PropertyReference{
			"Title":  {ID: "title-id", Name: "Title", Type: "title"},
			"Status": {ID: "status-id", Name: "Status", Type: "status"},
		},
	}

	idx := schema.NewIndex(ds)

	if id, ok := idx.IDForName("title"); !ok || id != "title-id" {
		t.Fatalf("IDForName(title) = %q,%v", id, ok)
	}
	if id, ok := idx.IDForName("STATUS"); !ok || id != "status-id" {
		t.Fatalf("IDForName(STATUS) = %q,%v", id, ok)
	}
	if name, ok := idx.NameForID("status-id"); !ok || name != "Status" {
		t.Fatalf("NameForID(status-id) = %q,%v", name, ok)
	}
	if _, ok := idx.IDForName("missing"); ok {
		t.Fatalf("expected missing property lookup to fail")
	}

	names := idx.PropertyNames()
	if len(names) != 2 || names[0] != "Status" || names[1] != "Title" {
		t.Fatalf("unexpected property names: %#v", names)
	}
}
