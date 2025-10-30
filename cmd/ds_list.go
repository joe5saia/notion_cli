package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/render"
)

func newDSListCmd(globals *globalOptions) *cobra.Command {
	var (
		databaseID string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List data sources within a database container",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if databaseID == "" {
				return fmt.Errorf("--database-id is required")
			}
			client, err := buildClient(globals.profile)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			dataSources, err := client.ListDataSources(ctx, databaseID)
			if err != nil {
				return fmt.Errorf("list data sources: %w", err)
			}

			switch format {
			case formatJSON:
				return render.JSON(cmd.OutOrStdout(), dataSources)
			case formatTable:
				headers := []string{"ID", "Name", "Type", "Properties"}
				return render.Table(cmd.OutOrStdout(), headers, dataSourceRows(dataSources))
			default:
				return fmt.Errorf("unknown format %q (expected json or table)", format)
			}
		},
	}

	cmd.Flags().StringVar(&databaseID, "database-id", "", "Notion database ID hosting the data sources")
	cmd.Flags().StringVar(&format, "format", formatTable, "Output format: json|table")

	return cmd
}

func dataSourceRows(sources []notion.DataSource) [][]string {
	rows := make([][]string, 0, len(sources))
	sort.SliceStable(sources, func(i, j int) bool {
		return sources[i].Name < sources[j].Name
	})
	for _, ds := range sources {
		rows = append(rows, []string{
			ds.ID,
			ds.Name,
			ds.DataSource,
			strconv.Itoa(len(ds.Properties)),
		})
	}
	return rows
}
