package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// NewCommand creates a command tree without global flag or output state.
func NewCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:           "goseed",
		Short:         "Generate Go business projects and modules",
		Long:          "goseed is under development. Help and version are available; project generation is not implemented yet.",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("project generation is not implemented yet; use --help for current capabilities")
		},
	}
}
