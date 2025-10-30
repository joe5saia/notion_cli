package cmd

import "github.com/spf13/cobra"

func newBlocksCmd(globals *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Block and content operations",
	}

	cmd.AddCommand(newBlocksAppendCmd(globals))

	return cmd
}
