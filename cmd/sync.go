package cmd

import "github.com/spf13/cobra"

func newSyncCmd(globals *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync automation helpers for watching Notion changes",
	}

	cmd.AddCommand(newSyncWatchCmd(globals))

	return cmd
}
