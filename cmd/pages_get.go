package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/expand"
	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/render"
)

type pagesGetOptions struct {
	format      string
	expandProps []string
}

func newPagesGetCmd(globals *globalOptions) *cobra.Command {
	opts := &pagesGetOptions{format: formatJSON}

	cmd := &cobra.Command{
		Use:   "get <page-id>",
		Short: "Retrieve a Notion page",
		Args:  cobra.ExactArgs(1),
		RunE:  opts.run(globals),
	}

	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: json|table")
	cmd.Flags().StringSliceVar(&opts.expandProps, "expand", nil, "Relation property names to expand")

	return cmd
}

func (opts *pagesGetOptions) run(globals *globalOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID := args[0]

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		page, err := opts.fetchPage(ctx, client, pageID)
		if err != nil {
			return err
		}

		page, err = opts.expandPage(ctx, client, page)
		if err != nil {
			return err
		}

		return opts.renderPage(cmd, page)
	}
}

func (opts *pagesGetOptions) fetchPage(ctx context.Context, client *notion.Client, pageID string) (notion.Page, error) {
	page, err := client.RetrievePage(ctx, pageID)
	if err != nil {
		return notion.Page{}, fmt.Errorf("retrieve page: %w", err)
	}
	return page, nil
}

func (opts *pagesGetOptions) expandPage(
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

func (opts *pagesGetOptions) renderPage(cmd *cobra.Command, page notion.Page) error {
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

func preparePageExpansion(page notion.Page, names []string) ([]notion.Page, []notion.PropertyReference, error) {
	refs := make([]notion.PropertyReference, 0, len(names))
	for _, name := range names {
		prop, ok := page.Properties[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown property %q", name)
		}
		if prop.Type != relationType {
			return nil, nil, fmt.Errorf("property %q is not a relation", name)
		}
		refs = append(refs, notion.PropertyReference{ID: prop.ID, Name: name, Type: prop.Type})
	}
	return []notion.Page{page}, refs, nil
}

func singlePageTable(page notion.Page) ([]string, [][]string) {
	headers := []string{"Field", "Value"}
	rows := [][]string{
		{"ID", page.ID},
		{"URL", page.URL},
	}
	if !page.LastEditedTime.IsZero() {
		rows = append(rows, []string{"Last Edited", page.LastEditedTime.UTC().Format(time.RFC3339)})
	}

	names := make([]string, 0, len(page.Properties))
	for name := range page.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		rows = append(rows, []string{name, summarizeProperty(page.Properties[name])})
	}

	return headers, rows
}
