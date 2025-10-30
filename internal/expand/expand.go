// Package expand implements helpers for expanding Notion relation properties.
package expand

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/yourorg/notionctl/internal/notion"
)

const (
	defaultConcurrency = 3
	relationType       = "relation"
)

// PageFetcher represents the subset of the Notion client used for relation expansion.
type PageFetcher interface {
	RetrievePage(ctx context.Context, pageID string) (notion.Page, error)
}

type relationRef struct {
	relationID string
	propertyID string
	pageIdx    int
}

// FirstLevel expands relation properties on the supplied pages using the provided property metadata.
func FirstLevel(
	ctx context.Context,
	client PageFetcher,
	pages []notion.Page,
	properties []notion.PropertyReference,
) error {
	if len(pages) == 0 || len(properties) == 0 {
		return nil
	}

	refs, ids, propByID := prepareRelationRefs(pages, properties)
	if len(refs) == 0 {
		return nil
	}

	relatedPages, err := fetchRelatedPages(ctx, client, ids)
	if err != nil {
		return err
	}

	applyExpandedRelations(pages, refs, propByID, relatedPages)
	return nil
}

func prepareRelationRefs(
	pages []notion.Page,
	properties []notion.PropertyReference,
) ([]relationRef, []string, map[string]notion.PropertyReference) {
	propByID := make(map[string]notion.PropertyReference, len(properties))
	for _, ref := range properties {
		if ref.Type == relationType {
			propByID[ref.ID] = ref
		}
	}

	uniqueIDs := map[string]struct{}{}
	refs := make([]relationRef, 0)

	for pageIdx := range pages {
		collectPageRelations(pageIdx, pages[pageIdx], properties, &refs, uniqueIDs)
	}

	ids := make([]string, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		ids = append(ids, id)
	}
	return refs, ids, propByID
}

func collectPageRelations(
	pageIdx int,
	page notion.Page,
	properties []notion.PropertyReference,
	refs *[]relationRef,
	unique map[string]struct{},
) {
	for _, ref := range properties {
		if ref.Type != relationType {
			continue
		}
		value, ok := page.Properties[ref.Name]
		if !ok || value.Type != relationType || len(value.Relation) == 0 {
			continue
		}
		for _, rel := range value.Relation {
			*refs = append(*refs, relationRef{
				relationID: rel.ID,
				propertyID: ref.ID,
				pageIdx:    pageIdx,
			})
			unique[rel.ID] = struct{}{}
		}
	}
}

func fetchRelatedPages(
	ctx context.Context,
	client PageFetcher,
	ids []string,
) (map[string]notion.Page, error) {
	if len(ids) == 0 {
		return map[string]notion.Page{}, nil
	}

	result := make(map[string]notion.Page, len(ids))
	var mu sync.Mutex

	sem := make(chan struct{}, defaultConcurrency)
	g, groupCtx := errgroup.WithContext(ctx)

	for _, id := range ids {
		relationID := id
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
			case <-groupCtx.Done():
				return groupCtx.Err()
			}
			defer func() { <-sem }()

			page, err := client.RetrievePage(groupCtx, relationID)
			if err != nil {
				return fmt.Errorf("retrieve related page %s: %w", relationID, err)
			}

			mu.Lock()
			result[relationID] = page
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("expand relations: %w", err)
	}

	return result, nil
}

func applyExpandedRelations(
	pages []notion.Page,
	refs []relationRef,
	propByID map[string]notion.PropertyReference,
	related map[string]notion.Page,
) {
	for _, ref := range refs {
		propRef, ok := propByID[ref.propertyID]
		if !ok {
			continue
		}
		relatedPage, ok := related[ref.relationID]
		if !ok {
			continue
		}
		if pages[ref.pageIdx].ExpandedRelations == nil {
			pages[ref.pageIdx].ExpandedRelations = make(map[string][]notion.Page)
		}
		bucket := pages[ref.pageIdx].ExpandedRelations[propRef.Name]
		if !containsPage(bucket, relatedPage.ID) {
			pages[ref.pageIdx].ExpandedRelations[propRef.Name] = append(bucket, relatedPage)
		}
	}
}

func containsPage(pages []notion.Page, id string) bool {
	for _, p := range pages {
		if p.ID == id {
			return true
		}
	}
	return false
}
