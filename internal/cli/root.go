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
	command := newProjectCommand("goseed")
	command.Short = "Generate Go business projects and modules"
	command.Long = "Generate a standalone Go business project with init, then add modules and resources with gen."
	command.Version = version
	generate := command.RunE
	command.RunE = func(cmd *cobra.Command, args []string) error {
		changed := cmd.Flags().NFlag()
		for _, name := range []string{"help", "version"} {
			if cmd.Flags().Changed(name) {
				changed--
			}
		}
		if changed == 0 {
			return cmd.Help()
		}
		return generate(cmd, args)
	}
	command.AddCommand(newProjectCommand("init"), newGenCommand())
	return command
}

func newProjectCommand(use string) *cobra.Command {
	var name, module, database, authorization, transport, deployment string
	var redis, metrics, tracing, dryRun bool
	command := &cobra.Command{
		Use:           use,
		Short:         "Generate a standalone Go project in the current directory",
		Long:          "Generate a standalone Go business project in the current directory.",
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
			options := templates.Options{Database: database, Redis: redis, Authz: authorization, Transport: transport, Metrics: metrics, Tracing: tracing, Deployment: deployment}
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
	flags := command.Flags()
	flags.StringVar(&name, "name", "", "project and service name (defaults to the current directory name)")
	flags.StringVar(&module, "module", "", "Go module path (defaults to example.com/<name>)")
	flags.StringVar(&database, "db", "sqlite", "database: sqlite, mysql, postgres, or none")
	flags.StringVar(&authorization, "authz", "none", "authorization: none or casbin")
	flags.BoolVar(&redis, "redis", false, "include Redis cache support")
	flags.StringVar(&transport, "transport", "http", "transport: http, grpc, dual, or gateway")
	flags.BoolVar(&metrics, "metrics", false, "include request and dependency metrics exported through OTLP/HTTP")
	flags.BoolVar(&tracing, "tracing", false, "include request tracing exported through OTLP/HTTP")
	flags.StringVar(&deployment, "deployment", "none", "deployment template: none or single-host")
	flags.BoolVar(&dryRun, "dry-run", false, "validate and display the generation plan without writing files")
	return command
}

func newGenCommand() *cobra.Command {
	gen := &cobra.Command{Use: "gen", Short: "Generate business modules and CRUD resources", Args: cobra.NoArgs}
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
	var spec string
	var migration, dryRun bool
	resource := &cobra.Command{
		Use:          "resource <name>",
		Short:        "Generate a CRUD resource from a project-local YAML spec",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			plan, err := generator.PrepareResource(root, args[0], generator.ResourceOptions{Spec: spec, Migration: migration})
			if err != nil {
				return err
			}
			if err := plan.Report(cmd.OutOrStdout(), dryRun); err != nil {
				return fmt.Errorf("write resource plan: %w", err)
			}
			if len(plan.Conflicts()) > 0 {
				return fmt.Errorf("generation conflicts; resolve them before generating")
			}
			if dryRun {
				return nil
			}
			if err := plan.Write(); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Generated resource %s in %s.\n", args[0], root)
			return err
		},
	}
	resource.Flags().StringVar(&spec, "spec", "", "project-relative YAML spec under specs/")
	resource.Flags().BoolVar(&dryRun, "dry-run", false, "validate and display the complete resource plan without writing files")
	resource.Flags().BoolVar(&migration, "migration", false, "include a SQL migration draft without applying it")
	_ = resource.MarkFlagRequired("spec")
	gen.AddCommand(resource)
	module.Flags().Bool("dry-run", false, "validate and display the module plan without writing files")
	gen.AddCommand(module)
	return gen
}
