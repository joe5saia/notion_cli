package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/brittonhayes/notionmd"
	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/notion"
)

type blocksAppendOptions struct {
	markdownPath string
}

func newBlocksAppendCmd(globals *globalOptions) *cobra.Command {
	opts := &blocksAppendOptions{}

	cmd := &cobra.Command{
		Use:   "append <block-or-page-id>",
		Short: "Append Markdown content as Notion blocks",
		Args:  cobra.ExactArgs(1),
		RunE:  opts.run(globals),
	}

	cmd.Flags().StringVar(&opts.markdownPath, "md", "", "Path to the Markdown file to append")

	return cmd
}

func (opts *blocksAppendOptions) run(globals *globalOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if opts.markdownPath == "" {
			return errors.New("--md is required")
		}

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		count, err := opts.appendMarkdown(ctx, client, args[0])
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Appended %d blocks\n", count); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func (opts *blocksAppendOptions) appendMarkdown(
	ctx context.Context,
	client *notion.Client,
	targetID string,
) (int, error) {
	blocks, err := loadMarkdownBlocks(opts.markdownPath)
	if err != nil {
		return 0, err
	}
	if len(blocks) == 0 {
		return 0, errors.New("no blocks generated from markdown")
	}

	if err := client.AppendBlockChildren(ctx, targetID, blocks); err != nil {
		return 0, fmt.Errorf("append blocks: %w", err)
	}
	return len(blocks), nil
}

func loadMarkdownBlocks(path string) ([]notion.Block, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- reading user-supplied markdown by design
	if err != nil {
		return nil, fmt.Errorf("read markdown: %w", err)
	}

	blocksJSON, err := notionmd.ConvertToJSON(string(data))
	if err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	encoded, err := json.Marshal(blocksJSON)
	if err != nil {
		return nil, fmt.Errorf("encode blocks: %w", err)
	}

	var blocks []notion.Block
	if err := json.Unmarshal(encoded, &blocks); err != nil {
		return nil, fmt.Errorf("decode blocks: %w", err)
	}

	return blocks, nil
}
