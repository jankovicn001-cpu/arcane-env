package environments

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"getarcane.dev/cli/v3/internal/cmdutil"
	"getarcane.dev/cli/v3/internal/output"
	clitypes "getarcane.dev/cli/v3/internal/types"
	"getarcane.dev/types/v3"
)

// GroupRef resolves an environment group by ID or by case-insensitive name.
var GroupRef = cmdutil.ResourceRef[types.EnvironmentGroup, types.EnvironmentGroup]{
	Singular: "environment",
	Plural:   "environments",
	IDHint:   "the environment ID",
	ListCmd:  "arcane environments list",
	GetPath:  func(_, identifier string) string { return clitypes.EnvironmentGroup(identifier) },
	ListPath: func(string) string { return clitypes.EnvironmentGroups() },
	Matches: func(item types.EnvironmentGroup, _, original string) bool {
		return strings.EqualFold(item.Name, original)
	},
	Label:   func(match types.EnvironmentGroup) string { return fmt.Sprintf("%s (%s)", match.Name, match.ID) },
	Promote: func(match types.EnvironmentGroup) *types.EnvironmentGroup { return &match },
}

func Command() *cobra.Command {
	command := &cobra.Command{
		Annotations: map[string]string{"arcane:scope": cmdutil.ScopeGlobal},
		Use:         "environments",
		Aliases:     []string{"environment", "env"},
		Short:       "Manage environments (groups of agents)",
	}
	command.AddCommand(newListCmd(), newCreateCmd(), newUpdateCmd(), newDeleteCmd())
	return command
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List environments",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[[]types.EnvironmentGroup]{
				Method:         http.MethodGet,
				Path:           clitypes.EnvironmentGroups(),
				FailureMessage: "failed to list environments",
				Render: func(data []types.EnvironmentGroup) error {
					if len(data) == 0 {
						output.Info("No environments found")
						return nil
					}
					rows := make([][]string, len(data))
					for i, group := range data {
						rows[i] = []string{group.ID, group.Name, group.Description, strconv.Itoa(group.EnvironmentCount)}
					}
					return output.Table([]string{"ID", "NAME", "DESCRIPTION", "AGENTS"}, rows)
				},
			})
		},
	}
}

func newCreateCmd() *cobra.Command {
	var opts struct {
		description string
	}
	command := &cobra.Command{
		Use:          "create <name>",
		Short:        "Create environment",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.EnvironmentGroup]{
				Method:         http.MethodPost,
				Path:           clitypes.EnvironmentGroups(),
				FailureMessage: "failed to create environment",
				Body:           types.CreateEnvironmentGroupRequest{Name: args[0], Description: opts.description},
				Render:         renderGroup("Environment created successfully"),
			})
		},
	}
	command.Flags().StringVar(&opts.description, "description", "", "Environment description")
	return command
}

func newUpdateCmd() *cobra.Command {
	var opts struct {
		name        string
		description string
	}
	command := &cobra.Command{
		Use:          "update <id-or-name>",
		Short:        "Update environment",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}
			group, _, err := GroupRef.Resolve(cmd.Context(), c, args[0], cmdutil.AllowPrompt(cmd))
			if err != nil {
				return err
			}

			req := map[string]any{}
			if cmd.Flags().Changed("name") {
				req["name"] = opts.name
			}
			if cmd.Flags().Changed("description") {
				req["description"] = opts.description
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.EnvironmentGroup]{
				Method:         http.MethodPatch,
				Path:           clitypes.EnvironmentGroup(group.ID),
				FailureMessage: "failed to update environment",
				Body:           req,
				Render:         renderGroup("Environment updated successfully"),
			})
		},
	}
	command.Flags().StringVar(&opts.name, "name", "", "Environment name")
	command.Flags().StringVar(&opts.description, "description", "", "Environment description")
	return command
}

func newDeleteCmd() *cobra.Command {
	var opts struct {
		force bool
	}
	command := &cobra.Command{
		Use:          "delete <id-or-name>",
		Aliases:      []string{"rm", "remove"},
		Short:        "Delete environment; its agents become unassigned",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}
			group, _, err := GroupRef.Resolve(cmd.Context(), c, args[0], cmdutil.AllowPrompt(cmd))
			if err != nil {
				return err
			}
			return cmdutil.RunDelete(cmd, c, cmdutil.DeleteSpec{
				Force:          opts.force,
				Prompt:         fmt.Sprintf("Are you sure you want to delete environment %s? Its agents become unassigned.", group.Name),
				Path:           clitypes.EnvironmentGroup(group.ID),
				FailureMessage: "failed to delete environment",
				SuccessMessage: "Environment deleted successfully",
			})
		},
	}
	command.Flags().BoolVarP(&opts.force, "force", "f", false, "Force deletion without confirmation")
	return command
}

func renderGroup(message string) func(types.EnvironmentGroup) error {
	return func(data types.EnvironmentGroup) error {
		output.Success("%s", message)
		output.KeyValue("ID", data.ID)
		output.KeyValue("Name", data.Name)
		output.KeyValue("Description", data.Description)
		output.KeyValue("Agents", data.EnvironmentCount)
		return nil
	}
}
