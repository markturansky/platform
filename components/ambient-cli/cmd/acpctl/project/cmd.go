// Package project implements project management commands
package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ambient-code/platform/components/ambient-cli/pkg/config"
	"github.com/ambient-code/platform/components/ambient-cli/pkg/connection"
	"github.com/ambient-code/platform/components/ambient-cli/pkg/output"
	sdktypes "github.com/ambient-code/platform/components/ambient-sdk/go-sdk/types"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "project [name|command]",
	Short: "Manage projects",
	Long:  `Manage projects in the Ambient Code Platform.`,
	Example: `  # Set current project context (shorthand)
  acpctl project my-project
  
  # Set current project context (explicit)  
  acpctl project set my-project
  
  # Get current project context  
  acpctl project current
  
  # List all projects
  acpctl project list`,
	Args: cobra.MaximumNArgs(1),
	RunE: projectMain,
}

var setCmd = &cobra.Command{
	Use:   "set <project-name>",
	Short: "Set the current project context",
	Args:  cobra.ExactArgs(1),
	RunE:  setProject,
}

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Display the current project context",
	Args:  cobra.NoArgs,
	RunE:  getCurrentProject,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Args:  cobra.NoArgs,
	RunE:  listProjects,
}

func init() {
	Cmd.AddCommand(setCmd)
	Cmd.AddCommand(currentCmd)
	Cmd.AddCommand(listCmd)
}

func projectMain(cmd *cobra.Command, args []string) error {
	// If no args, show current project
	if len(args) == 0 {
		return getCurrentProject(cmd, args)
	}

	// If one arg, treat it as setting the project
	projectName := args[0]
	return setProject(cmd, []string{projectName})
}

func setProject(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	// Validate project name against Kubernetes DNS label requirements
	if err := validateProjectName(projectName); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	client, err := connection.NewClientFromConfig()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetRequestTimeout())
	defer cancel()

	// Try to get the project first
	project, err := client.Projects().Get(ctx, projectName)
	if err != nil {
		// Only attempt to create if this is a 404 (not found) error
		if !isNotFoundError(err) {
			return fmt.Errorf("failed to get project %q: %w", projectName, err)
		}

		// Project doesn't exist, try to create it
		newProject, buildErr := sdktypes.NewProjectBuilder().
			Name(projectName).
			DisplayName(projectName).
			Description(fmt.Sprintf("Project %s", projectName)).
			Build()
		if buildErr != nil {
			return fmt.Errorf("build project: %w", buildErr)
		}

		project, err = client.Projects().Create(ctx, newProject)
		if err != nil {
			// Check if error is "project already exists" (409) - race condition
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
				// Project exists, create a minimal project object for config purposes
				project = &sdktypes.Project{
					Name: projectName,
				}
			} else {
				return fmt.Errorf("create project %q: %w", projectName, err)
			}
		}
	}

	// Save the project in config
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Project = project.Name

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Switched to project %q\n", project.Name)
	return nil
}

func getCurrentProject(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	currentProject := cfg.GetProject()
	if currentProject == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "No project context set")
		fmt.Fprintln(cmd.OutOrStdout(), "Use 'acpctl project set <project-name>' to set a project context")
		return nil
	}

	// Show where the project setting comes from
	if env := os.Getenv("AMBIENT_PROJECT"); env != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Current project: %s (from AMBIENT_PROJECT environment variable)\n", currentProject)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Current project: %s\n", currentProject)
	}
	return nil
}

func listProjects(cmd *cobra.Command, args []string) error {
	client, err := connection.NewClientFromConfig()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetRequestTimeout())
	defer cancel()

	listOpts := sdktypes.NewListOptions().Size(100).Build()
	list, err := client.Projects().List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	if len(list.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects found")
		return nil
	}

	return printProjectTable(list.Items)
}

func printProjectTable(projects []sdktypes.Project) error {
	columns := []output.Column{
		{Name: "NAME", Width: 30},
		{Name: "DESCRIPTION", Width: 50},
		{Name: "AGE", Width: 10},
	}

	printer := output.NewPrinter(output.FormatTable)
	table := output.NewTable(printer.Writer(), columns)
	table.WriteHeaders()

	for _, p := range projects {
		age := ""
		if p.CreatedAt != nil {
			age = output.FormatAge(time.Since(*p.CreatedAt))
		}
		description := p.Description
		table.WriteRow(p.Name, description, age)
	}
	return nil
}

// isNotFoundError checks if an error is a 404 Not Found error using structured error types
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for structured API error with 404 status code
	var apiErr *sdktypes.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}

	return false
}

// validateProjectName validates project name against Kubernetes DNS label requirements
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("project name must be 63 characters or less")
	}

	// Kubernetes DNS label regex: lowercase alphanumeric and hyphens,
	// must start and end with alphanumeric
	validPattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("project name must contain only lowercase letters, numbers, and hyphens, and must start and end with an alphanumeric character")
	}

	return nil
}
