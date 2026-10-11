package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/setcreed/goseed/internal/generator"
	"github.com/setcreed/goseed/internal/templates"
	"github.com/spf13/cobra"
)

// NewCommand creates a command tree without global flag or output state.
func NewCommand(version string) *cobra.Command {
	var name, module, database, authorization, transport string
	var redis, dryRun bool
	command := &cobra.Command{
		Use:           "goseed",
		Short:         "Generate Go business projects and modules",
		Long:          "Generate a standalone Go business project in the current directory.",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			projectName := name
			if projectName == "" {
				projectName = filepath.Base(root)
			}
			modulePath := module
			if modulePath == "" {
				modulePath = "example.com/" + projectName
			}
			options := templates.Options{Database: database, Redis: redis, Authz: authorization, Transport: transport}
			plan, err := generator.Prepare(root, templates.Project{Name: projectName, Module: modulePath}, options)
			if err != nil {
				return err
			}
			if err := plan.Report(cmd.OutOrStdout(), dryRun); err != nil {
				return fmt.Errorf("write generation plan: %w", err)
			}
			if len(plan.Conflicts) > 0 {
				return fmt.Errorf("generation conflicts; resolve them before generating")
			}
			if dryRun {
				return nil
			}
			if err := plan.Write(); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Generated %s in %s.\n", projectName, root)
			return err
		},
	}
	command.AddCommand(newGenCommand())
	flags := command.Flags()
	flags.StringVar(&name, "name", "", "project and service name (defaults to the current directory name)")
	flags.StringVar(&module, "module", "", "Go module path (defaults to example.com/<name>)")
	flags.StringVar(&database, "db", "sqlite", "database: sqlite, mysql, postgres, or none")
	flags.StringVar(&authorization, "authz", "none", "authorization: none or casbin")
	flags.BoolVar(&redis, "redis", false, "include Redis cache support")
	flags.StringVar(&transport, "transport", "http", "transport: http, grpc, dual, or gateway")
	flags.BoolVar(&dryRun, "dry-run", false, "validate and display the generation plan without writing files")
	return command
}

func newGenCommand() *cobra.Command {
	gen := &cobra.Command{Use: "gen", Short: "Generate business modules", Args: cobra.NoArgs}
	module := &cobra.Command{
		Use:          "module <name>",
		Short:        "Generate a behavior-free business module skeleton",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			plan, err := generator.PrepareModule(root, args[0])
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			if err := plan.Report(cmd.OutOrStdout(), dryRun); err != nil {
				return fmt.Errorf("write module plan: %w", err)
			}
			if len(plan.Conflicts) > 0 {
				return fmt.Errorf("generation conflicts; resolve them before generating")
			}
			if dryRun {
				return nil
			}
			if err := plan.Write(); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Generated module %s in %s.\n", args[0], root)
			return err
		},
	}
	module.Flags().Bool("dry-run", false, "validate and display the module plan without writing files")
	gen.AddCommand(module)
	return gen
}
