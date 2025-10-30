// Package schema provides helpers for working with Notion data source schemas.
package schema

import (
	"sort"
	"strings"

	"github.com/yourorg/notionctl/internal/notion"
)

// Index accelerates lookups between property names and IDs.
type Index struct {
	byName map[string]notion.PropertyReference
	byID   map[string]notion.PropertyReference
	order  []string
}

// NewIndex builds a property index from a data source definition.
func NewIndex(ds notion.DataSource) *Index {
	byName := make(map[string]notion.PropertyReference, len(ds.Properties))
	byID := make(map[string]notion.PropertyReference, len(ds.Properties))
	names := make([]string, 0, len(ds.Properties))

	for name, ref := range ds.Properties {
		byID[ref.ID] = ref
		key := normalize(name)
		byName[key] = ref
		names = append(names, name)
	}

	sort.Strings(names)

	return &Index{
		byName: byName,
		byID:   byID,
		order:  names,
	}
}

// IDForName resolves a property name (case-insensitive) to its property ID.
func (i *Index) IDForName(name string) (string, bool) {
	if i == nil {
		return "", false
	}
	ref, ok := i.byName[normalize(name)]
	if !ok {
		return "", false
	}
	return ref.ID, true
}

// NameForID returns the display name for a property ID if present.
func (i *Index) NameForID(id string) (string, bool) {
	if i == nil {
		return "", false
	}
	ref, ok := i.byID[id]
	if !ok {
		return "", false
	}
	return ref.Name, true
}

// ReferenceForName returns the full property reference.
func (i *Index) ReferenceForName(name string) (notion.PropertyReference, bool) {
	if i == nil {
		return notion.PropertyReference{}, false
	}
	ref, ok := i.byName[normalize(name)]
	return ref, ok
}

// ReferenceForID returns the full property reference.
func (i *Index) ReferenceForID(id string) (notion.PropertyReference, bool) {
	if i == nil {
		return notion.PropertyReference{}, false
	}
	ref, ok := i.byID[id]
	return ref, ok
}

// PropertyNames returns the sorted property names for deterministic output.
func (i *Index) PropertyNames() []string {
	if i == nil {
		return nil
	}
	out := make([]string, len(i.order))
	copy(out, i.order)
	return out
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
