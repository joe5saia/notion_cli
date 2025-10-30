package cmd

import "github.com/spf13/cobra"

func newDSCmd(globals *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ds",
		Short: "Data source operations",
	}

	cmd.AddCommand(newDSListCmd(globals))
	cmd.AddCommand(newDSQueryCmd(globals))

	return cmd
}
