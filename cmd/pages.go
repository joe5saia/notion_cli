package cmd

import "github.com/spf13/cobra"

func newPagesCmd(globals *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Work with Notion pages",
	}

	cmd.AddCommand(newPagesGetCmd(globals))
	cmd.AddCommand(newPagesUpdateCmd(globals))

	return cmd
}
