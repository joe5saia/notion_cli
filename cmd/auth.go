package cmd

import "github.com/spf13/cobra"

func newAuthCmd(globals *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Notion authentication profiles",
	}

	cmd.AddCommand(newAuthLoginCmd(globals))

	return cmd
}
