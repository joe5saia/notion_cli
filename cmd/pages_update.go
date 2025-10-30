package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/expand"
	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/render"
)

type pagesUpdateOptions struct {
	propsPath        string
	format           string
	expandProps      []string
	replaceRelations bool
	archive          bool
}

func newPagesUpdateCmd(globals *globalOptions) *cobra.Command {
	opts := &pagesUpdateOptions{format: formatJSON}

	cmd := &cobra.Command{
		Use:   "update <page-id>",
		Short: "Update a Notion page's properties",
		Args:  cobra.ExactArgs(1),
		RunE:  opts.run(globals),
	}

	cmd.Flags().StringVar(&opts.propsPath, "props", "", "Path to JSON file describing property updates")
	cmd.Flags().BoolVar(
		&opts.replaceRelations,
		"replace-relations",
		false,
		"Replace relation properties instead of merging with existing values",
	)
	cmd.Flags().StringSliceVar(&opts.expandProps, "expand", nil, "Relation property names to expand after update")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: json|table")
	cmd.Flags().BoolVar(&opts.archive, "archive", false, "Archive or unarchive the page")

	return cmd
}

func (opts *pagesUpdateOptions) run(globals *globalOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := opts.validate(); err != nil {
			return err
		}

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		pageID := args[0]

		archiveSet := cmd.Flags().Changed("archive")
		updated, err := opts.applyUpdates(ctx, client, pageID, archiveSet)
		if err != nil {
			return err
		}

		updated, err = opts.expandPage(ctx, client, updated)
		if err != nil {
			return err
		}

		return opts.renderPage(cmd, updated)
	}
}

func (opts *pagesUpdateOptions) validate() error {
	if opts.propsPath == "" {
		return errors.New("--props is required")
	}
	return nil
}

func (opts *pagesUpdateOptions) applyUpdates(
	ctx context.Context,
	client *notion.Client,
	pageID string,
	archiveSet bool,
) (notion.Page, error) {
	existing, err := client.RetrievePage(ctx, pageID)
	if err != nil {
		return notion.Page{}, fmt.Errorf("retrieve page: %w", err)
	}

	updates, err := loadUpdatePayload(opts.propsPath)
	if err != nil {
		return notion.Page{}, err
	}

	if mergeErr := mergeRelationProperties(existing, updates, opts.replaceRelations); mergeErr != nil {
		return notion.Page{}, mergeErr
	}

	req := notion.UpdatePageRequest{Properties: updates}
	if archiveSet {
		req.Archived = &opts.archive
	}

	updated, err := client.UpdatePage(ctx, pageID, req)
	if err != nil {
		return notion.Page{}, fmt.Errorf("update page: %w", err)
	}
	return updated, nil
}

func (opts *pagesUpdateOptions) expandPage(
	ctx context.Context,
	client expand.PageFetcher,
	page notion.Page,
) (notion.Page, error) {
	if len(opts.expandProps) == 0 {
		return page, nil
	}
	pages, refs, err := preparePageExpansion(page, opts.expandProps)
	if err != nil {
		return notion.Page{}, err
	}
	if err := expand.FirstLevel(ctx, client, pages, refs); err != nil {
		return notion.Page{}, fmt.Errorf("expand relations: %w", err)
	}
	return pages[0], nil
}

func (opts *pagesUpdateOptions) renderPage(cmd *cobra.Command, page notion.Page) error {
	switch opts.format {
	case formatJSON:
		if err := render.JSON(cmd.OutOrStdout(), page); err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		return nil
	case formatTable:
		headers, rows := singlePageTable(page)
		if err := render.Table(cmd.OutOrStdout(), headers, rows); err != nil {
			return fmt.Errorf("render table: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q (expected json or table)", opts.format)
	}
}

func loadUpdatePayload(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- reading user-specified update payload is intended
	if err != nil {
		return nil, fmt.Errorf("read props: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode props: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("property payload is empty")
	}
	return payload, nil
}

func mergeRelationProperties(
	existing notion.Page,
	updates map[string]any,
	replace bool,
) error {
	for name, raw := range updates {
		existingValue, ok := existing.Properties[name]
		if !ok || existingValue.Type != relationType {
			continue
		}

		updateMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		newRelations, ok := updateMap["relation"].([]any)
		if !ok {
			continue
		}

		merged, err := mergeRelationArray(existingValue, newRelations, replace)
		if err != nil {
			return fmt.Errorf("merge relations for %s: %w", name, err)
		}
		updateMap["relation"] = merged
		updates[name] = updateMap
	}
	return nil
}

func mergeRelationArray(
	existing notion.PropertyValue,
	updates []any,
	replace bool,
) ([]map[string]string, error) {
	if replace {
		return normalizeRelationArray(updates)
	}

	dedupe := map[string]struct{}{}
	for _, rel := range existing.Relation {
		dedupe[rel.ID] = struct{}{}
	}

	normalized, err := normalizeRelationArray(updates)
	if err != nil {
		return nil, err
	}

	for _, rel := range normalized {
		dedupe[rel["id"]] = struct{}{}
	}

	ids := make([]string, 0, len(dedupe))
	for id := range dedupe {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	merged := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		merged = append(merged, map[string]string{"id": id})
	}
	return merged, nil
}

func normalizeRelationArray(items []any) ([]map[string]string, error) {
	normalized := make([]map[string]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("relation entry must be an object, got %T", item)
		}
		idRaw, ok := m["id"].(string)
		if !ok || idRaw == "" {
			return nil, errors.New("relation entry missing id")
		}
		normalized = append(normalized, map[string]string{"id": idRaw})
	}
	return normalized, nil
}
