package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/schema"
)

type changesOptions struct {
	dsOpts       *dsQueryOptions
	since        time.Time
	until        time.Time
	dataSourceID string
}

func newChangesCmd(globals *globalOptions) *cobra.Command {
	opts := &changesOptions{dsOpts: &dsQueryOptions{format: formatJSON, fetchAll: true}}

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "List changes for a data source over a time window",
		RunE:  opts.run(globals),
	}

	cmd.Flags().StringVar(&opts.dataSourceID, "data-source-id", "", "Target data source ID")
	cmd.Flags().StringVar(&opts.dsOpts.format, "format", opts.dsOpts.format, "Output format: json|table")
	cmd.Flags().StringSliceVar(&opts.dsOpts.expandRelations, "expand", nil, "Relation property names to expand")
	cmd.Flags().String("since", "", "Start of time window (RFC3339)")
	cmd.Flags().String("until", "", "End of time window (RFC3339)")
	cobra.CheckErr(cmd.MarkFlagRequired("data-source-id"))
	cobra.CheckErr(cmd.MarkFlagRequired("since"))

	return cmd
}

func (opts *changesOptions) run(globals *globalOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if err := opts.parseWindow(cmd); err != nil {
			return err
		}

		opts.dsOpts.dataSourceID = opts.dataSourceID
		if err := opts.prepareQuery(); err != nil {
			return err
		}

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		resp, index, err := opts.executeQuery(ctx, client)
		if err != nil {
			return err
		}

		return opts.dsOpts.renderResults(cmd, resp, index)
	}
}

func (opts *changesOptions) prepareQuery() error {
	filter, err := buildChangesFilter(opts.since, opts.until)
	if err != nil {
		return err
	}
	opts.dsOpts.filterJSON = filter

	sorts, err := buildLastEditedSort()
	if err != nil {
		return err
	}
	opts.dsOpts.sortsJSON = sorts
	opts.dsOpts.fetchAll = true
	return nil
}

func (opts *changesOptions) executeQuery(
	ctx context.Context,
	client *notion.Client,
) (notion.QueryDataSourceResponse, *schema.Index, error) {
	if validateErr := opts.dsOpts.validate(); validateErr != nil {
		return notion.QueryDataSourceResponse{}, nil, validateErr
	}
	return opts.dsOpts.executeQuery(ctx, client)
}

func (opts *changesOptions) parseWindow(cmd *cobra.Command) error {
	sinceStr, err := cmd.Flags().GetString("since")
	if err != nil {
		return fmt.Errorf("read --since: %w", err)
	}
	untilStr, err := cmd.Flags().GetString("until")
	if err != nil {
		return fmt.Errorf("read --until: %w", err)
	}
	if sinceStr == "" {
		return errors.New("--since is required")
	}
	since, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}
	opts.since = since.UTC()
	if untilStr != "" {
		until, err := time.Parse(time.RFC3339, untilStr)
		if err != nil {
			return fmt.Errorf("parse --until: %w", err)
		}
		opts.until = until.UTC()
	} else {
		opts.until = time.Now().UTC()
	}
	if opts.until.Before(opts.since) {
		return errors.New("--until must be after --since")
	}
	return nil
}

func buildChangesFilter(since, until time.Time) (string, error) {
	filter := map[string]any{
		"timestamp": "last_edited_time",
		"last_edited_time": map[string]any{
			"on_or_after":  since.Format(time.RFC3339),
			"on_or_before": until.Format(time.RFC3339),
		},
	}
	data, err := json.Marshal(filter)
	if err != nil {
		return "", fmt.Errorf("build filter: %w", err)
	}
	return string(data), nil
}

func buildLastEditedSort() (string, error) {
	sorts := []map[string]any{
		{
			"timestamp": "last_edited_time",
			"direction": "descending",
		},
	}
	data, err := json.Marshal(sorts)
	if err != nil {
		return "", fmt.Errorf("build sorts: %w", err)
	}
	return string(data), nil
}
