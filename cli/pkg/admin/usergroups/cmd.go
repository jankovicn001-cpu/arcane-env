package usergroups

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

// groupRef resolves a user group by ID or by case-insensitive name.
var groupRef = cmdutil.ResourceRef[types.UserGroup, types.UserGroup]{
	Singular: "user group",
	Plural:   "user groups",
	IDHint:   "the user group ID",
	ListCmd:  "arcane admin user-groups list",
	GetPath:  func(_, identifier string) string { return clitypes.UserGroup(identifier) },
	ListPath: func(string) string { return clitypes.UserGroups() },
	Matches: func(item types.UserGroup, _, original string) bool {
		return strings.EqualFold(item.Name, original)
	},
	Label:   func(match types.UserGroup) string { return fmt.Sprintf("%s (%s)", match.Name, match.ID) },
	Promote: func(match types.UserGroup) *types.UserGroup { return &match },
}

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:     "user-groups",
		Aliases: []string{"user-group", "usergroups", "ug"},
		Short:   "Manage user groups",
		Long:    "Manage user groups. Members and role assignments are managed in the web UI.",
	}
	command.AddCommand(newListCmd(), newCreateCmd(), newDeleteCmd())
	return command
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List user groups",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[[]types.UserGroup]{
				Method:         http.MethodGet,
				Path:           clitypes.UserGroups(),
				FailureMessage: "failed to list user groups",
				Render: func(data []types.UserGroup) error {
					if len(data) == 0 {
						output.Info("No user groups found")
						return nil
					}
					rows := make([][]string, len(data))
					for i, group := range data {
						rows[i] = []string{group.ID, group.Name, group.Description, strconv.Itoa(group.MemberCount), linkSummary(group.Links)}
					}
					return output.Table([]string{"ID", "NAME", "DESCRIPTION", "MEMBERS", "LINKS"}, rows)
				},
			})
		},
	}
}

func newCreateCmd() *cobra.Command {
	var opts struct {
		description string
		oidc        []string
		ldap        []string
	}
	command := &cobra.Command{
		Use:          "create <name>",
		Short:        "Create user group",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			links := make([]types.UserGroupLink, 0, len(opts.oidc)+len(opts.ldap))
			for _, value := range opts.oidc {
				links = append(links, types.UserGroupLink{Kind: "oidc", Value: value})
			}
			for _, value := range opts.ldap {
				links = append(links, types.UserGroupLink{Kind: "ldap", Value: value})
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.UserGroup]{
				Method:         http.MethodPost,
				Path:           clitypes.UserGroups(),
				FailureMessage: "failed to create user group",
				Body:           types.CreateUserGroupRequest{Name: args[0], Description: opts.description, Links: links},
				Render: func(data types.UserGroup) error {
					output.Success("User group created successfully")
					output.KeyValue("ID", data.ID)
					output.KeyValue("Name", data.Name)
					output.KeyValue("Description", data.Description)
					output.KeyValue("Links", linkSummary(data.Links))
					return nil
				},
			})
		},
	}
	command.Flags().StringVar(&opts.description, "description", "", "User group description")
	command.Flags().StringArrayVar(&opts.oidc, "oidc", nil, "SSO claim value whose users join the group (repeatable)")
	command.Flags().StringArrayVar(&opts.ldap, "ldap", nil, "LDAP group name whose users join the group (repeatable)")
	return command
}

func newDeleteCmd() *cobra.Command {
	var opts struct {
		force bool
	}
	command := &cobra.Command{
		Use:          "delete <id-or-name>",
		Aliases:      []string{"rm", "remove"},
		Short:        "Delete user group with its members and role assignments",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}
			group, _, err := groupRef.Resolve(cmd.Context(), c, args[0], cmdutil.AllowPrompt(cmd))
			if err != nil {
				return err
			}
			return cmdutil.RunDelete(cmd, c, cmdutil.DeleteSpec{
				Force:          opts.force,
				Prompt:         fmt.Sprintf("Are you sure you want to delete user group %s? Its members lose the access it grants.", group.Name),
				Path:           clitypes.UserGroup(group.ID),
				FailureMessage: "failed to delete user group",
				SuccessMessage: "User group deleted successfully",
			})
		},
	}
	command.Flags().BoolVarP(&opts.force, "force", "f", false, "Force deletion without confirmation")
	return command
}

func linkSummary(links []types.UserGroupLink) string {
	if len(links) == 0 {
		return "—"
	}
	parts := make([]string, len(links))
	for i, link := range links {
		parts[i] = link.Kind + ":" + link.Value
	}
	return strings.Join(parts, ", ")
}
