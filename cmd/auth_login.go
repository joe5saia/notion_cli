package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yourorg/notionctl/internal/config"
)

type loginOptions struct {
	notionVersion string
	token         string
	oauth         bool
}

const notionVersionFlagHelp = "Override the Notion API version for the profile"

func newAuthLoginCmd(globals *globalOptions) *cobra.Command {
	opts := &loginOptions{
		notionVersion: config.DefaultNotionVersion(),
	}

	cmd := &cobra.Command{
		Use:           "login",
		Short:         "Store a Notion integration token securely",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthLogin(cmd, globals, opts)
		},
	}

	cmd.Flags().StringVar(&opts.token, "token", "", "Notion integration token to store (prompted if omitted)")
	cmd.Flags().BoolVar(&opts.oauth, "oauth", false, "Use OAuth device flow instead of a manual token")
	cmd.Flags().StringVar(
		&opts.notionVersion,
		"notion-version",
		opts.notionVersion,
		notionVersionFlagHelp,
	)

	return cmd
}

func runAuthLogin(cmd *cobra.Command, globals *globalOptions, opts *loginOptions) error {
	if opts.oauth {
		return errors.New("oauth login flow is not implemented yet; supply --token")
	}

	token := strings.TrimSpace(opts.token)
	if token == "" {
		read, err := promptForToken(cmd)
		if err != nil {
			return err
		}
		token = read
	}
	if token == "" {
		return errors.New("token cannot be empty")
	}

	version := strings.TrimSpace(opts.notionVersion)
	if version == "" {
		version = config.DefaultNotionVersion()
	}

	if err := config.SaveToken(globals.profile, token, version); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Saved credentials for profile %q (Notion-Version %s)\n",
		globals.profile,
		version,
	); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}
	return nil
}

func promptForToken(cmd *cobra.Command) (string, error) {
	reader := cmd.InOrStdin()

	if f, ok := reader.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if _, err := fmt.Fprint(cmd.OutOrStdout(), "Notion token: "); err != nil {
			return "", fmt.Errorf("prompt token: %w", err)
		}
		data, err := term.ReadPassword(int(f.Fd()))
		if _, ferr := fmt.Fprintln(cmd.OutOrStdout()); ferr != nil {
			return "", fmt.Errorf("prompt token: %w", ferr)
		}
		if err != nil {
			return "", fmt.Errorf("read token: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
