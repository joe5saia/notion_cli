package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type globalOptions struct {
	profile string
}

var globals = &globalOptions{
	profile: "default",
}

var rootCmd = &cobra.Command{
	Use:           "notionctl",
	Short:         "CLI for working with the modern Notion API",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the command hierarchy.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("execute command: %w", err)
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&globals.profile, "profile", globals.profile, "Auth profile to use")

	rootCmd.SetErr(os.Stderr)
	rootCmd.SetOut(os.Stdout)

	rootCmd.AddCommand(newAuthCmd(globals))
	rootCmd.AddCommand(newDSCmd(globals))
	rootCmd.AddCommand(newPagesCmd(globals))
	rootCmd.AddCommand(newBlocksCmd(globals))
	rootCmd.AddCommand(newChangesCmd(globals))
	rootCmd.AddCommand(newSyncCmd(globals))
}
