package podman

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/spf13/cobra"

	"kit.getarcane.app"

	"getarcane.dev/cli/v3/internal/client"
	"getarcane.dev/cli/v3/internal/cmdutil"
	"getarcane.dev/cli/v3/internal/output"
	internaltypes "getarcane.dev/cli/v3/internal/types"
	"getarcane.dev/types/v3"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "podman",
		Aliases: []string{"pm"},
		Short:   "Manage Podman pods, quadlets, kube YAML, and secrets",
	}
	cmd.AddCommand(
		newStatusCmd(),
		newPodsCmd(),
		newQuadletsCmd(),
		newKubeCmd(),
		newSecretsCmd(),
	)
	cmdutil.SetScope(cmdutil.ScopeResource, cmd)
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show Podman detection and capabilities",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.PodmanStatus]{
				Method:         http.MethodGet,
				Path:           internaltypes.PodmanStatus(c.EnvID()),
				FailureMessage: "failed to get Podman status",
				Render: func(data types.PodmanStatus) error {
					if !data.Detected {
						output.Info("Podman was not detected on this agent")
						return nil
					}
					output.Header("Podman Status")
					output.KeyValue("Version", data.Version)
					output.KeyValue("API version", data.APIVersion)
					output.KeyValue("Rootless", kit.Ternary(data.Rootless, "Yes", "No"))
					output.KeyValue("Quadlets", kit.Ternary(data.QuadletsSupported, "Supported", "Unsupported"))
					output.KeyValue("Quadlet apps", kit.Ternary(data.QuadletAppsSupported, "Supported", "Unsupported"))
					output.KeyValue("systemd", kit.Ternary(data.SystemdAvailable, string(data.SystemdScope), "Unavailable"))
					if data.SystemdReason != "" {
						output.KeyValue("systemd reason", data.SystemdReason)
					}
					return nil
				},
			})
		},
	}
	cmdutil.SetScope(cmdutil.ScopeRead, cmd)
	return cmd
}

func searchPodman[S any](ctx context.Context, c *client.Client, path string, matches func(S) bool) ([]S, error) {
	result, err := c.GetJSON[[]S](ctx, fmt.Sprintf("%s?limit=%d", path, cmdutil.ShowAllLimit))
	if statusErr, ok := errors.AsType[*client.StatusError](err); ok && statusErr.StatusCode == http.StatusBadRequest {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(result.Data, func(item S) bool {
		return !matches(item)
	}), nil
}
