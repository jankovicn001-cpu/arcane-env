package system

import (
	"encoding/json/v2"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"kit.getarcane.app"

	"getarcane.dev/cli/v3/internal/cmdutil"
	"getarcane.dev/cli/v3/internal/output"
	clitypes "getarcane.dev/cli/v3/internal/types"
	"getarcane.dev/types/v3"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "system",
		Aliases: []string{"sys"},
		Short:   "System operations",
	}
	cmd.AddCommand(
		newPruneCmd(),
		newDockerInfoCmd(),
		newSystemContainersCmd(),
		newConvertCmd(),
	)
	return cmd
}

func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "prune",
		Short:        "Prune all unused resources",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			confirmed, confirmationErr := cmdutil.ConfirmOrCancel(cmd, cmdutil.ConfirmOptions{Label: "Are you sure you want to prune all unused resources?"})
			if confirmationErr != nil || !confirmed {
				return confirmationErr
			}
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}
			return cmdutil.Run(cmd, c, cmdutil.Spec[types.PruneAllResult]{
				Method:         http.MethodPost,
				Path:           clitypes.SystemPrune(c.EnvID()),
				FailureMessage: "failed to prune",
				Body: types.PruneAllRequest{
					Containers: &types.PruneContainersOptions{Mode: types.PruneContainerModeStopped},
					Images:     &types.PruneImagesOptions{Mode: types.PruneImageModeDangling},
					Networks:   &types.PruneNetworksOptions{Mode: types.PruneNetworkModeUnused},
				},
				Render: func(data types.PruneAllResult) error {
					output.Header("System Prune Results")
					output.KeyValue("Space Reclaimed", output.UnsignedBytes(data.SpaceReclaimed))
					return nil
				},
			})
		},
	}
	cmdutil.SetScope(cmdutil.ScopeAll, cmd)
	return cmd
}

func newDockerInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "info",
		Aliases:      []string{"docker-info"},
		Short:        "Get Docker daemon information",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			// This endpoint returns dockerinfo.Info directly, not wrapped in an
			// ApiResponse envelope.
			body, err := c.DoRaw(cmd.Context(), http.MethodGet, clitypes.SystemDockerInfo(c.EnvID()), nil)
			if err != nil {
				return fmt.Errorf("failed to get docker info: %w", err)
			}

			if cmdutil.JSONOutputEnabled(cmd) {
				return cmdutil.PrintRawJSON(body)
			}
			var result types.DockerInfo
			if decodeErr := json.Unmarshal(body, &result); decodeErr != nil {
				return fmt.Errorf("failed to get docker info: failed to decode response: %w", decodeErr)
			}

			output.Header("Docker Info")
			output.KeyValue("API Version", result.APIVersion)
			output.KeyValue("OS", result.Os)
			output.KeyValue("Architecture", result.Arch)
			output.KeyValue("Go Version", result.GoVersion)
			return nil
		},
	}
	cmdutil.SetScope(cmdutil.ScopeRead, cmd)
	return cmd
}

func newSystemContainersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Bulk container operations",
	}
	cmd.AddCommand(
		newContainersStartCmd(),
		newContainersStopCmd(),
	)
	return cmd
}

func newContainersStartCmd() *cobra.Command {
	var opts struct {
		startStoppedOnlyFlag bool
	}
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start all containers",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.ContainerActionResult]{
				Method:         http.MethodPost,
				Path:           clitypes.SystemContainersAction(c.EnvID(), kit.Ternary(opts.startStoppedOnlyFlag, "startstopped", "startall")),
				FailureMessage: "failed to start containers",
				Render: func(types.ContainerActionResult) error {
					output.Success("%s", kit.Ternary(opts.startStoppedOnlyFlag, "Started all stopped containers", "Started all containers"))
					return nil
				},
			})
		},
	}
	cmd.Flags().BoolVar(&opts.startStoppedOnlyFlag, "stopped", false, "Only start containers that are currently stopped")
	cmdutil.SetScope(cmdutil.ScopeAll, cmd)
	return cmd
}

func newContainersStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "stop",
		Short:        "Stop all containers",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			return cmdutil.Run(cmd, c, cmdutil.Spec[types.ContainerActionResult]{
				Method:         http.MethodPost,
				Path:           clitypes.SystemContainersAction(c.EnvID(), "stopall"),
				FailureMessage: "failed to stop all containers",
				Render:         func(types.ContainerActionResult) error { output.Success("%s", "Stopped all containers"); return nil },
			})
		},
	}
	cmdutil.SetScope(cmdutil.ScopeAll, cmd)
	return cmd
}

func newConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "convert <docker-run-command>",
		Short:        "Convert docker run command to compose",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cmdutil.ClientFromCommand(cmd)
			if err != nil {
				return err
			}

			resp, err := c.Request(cmd.Context(), http.MethodPost, clitypes.SystemConvert(c.EnvID()), types.ConvertDockerRunRequest{DockerRunCommand: args[0]})
			if err != nil {
				return fmt.Errorf("failed to convert command: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, readErr := cmdutil.ReadJSONBody(resp)
			if readErr != nil {
				return fmt.Errorf("failed to convert command: %w", readErr)
			}
			if cmdutil.JSONOutputEnabled(cmd) {
				return cmdutil.PrintRawJSON(body)
			}
			var result types.ConvertDockerRunResponse
			if decodeErr := json.Unmarshal(body, &result); decodeErr != nil {
				return fmt.Errorf("failed to convert command: failed to decode response: %w", decodeErr)
			}

			output.Header("Conversion Result")
			if result.ServiceName != "" {
				_, _ = fmt.Fprintf(output.Stdout(), "Service: %s\n\n", result.ServiceName)
			}
			if result.DockerCompose != "" {
				_, _ = fmt.Fprintln(output.Stdout(), "Docker Compose:")
				_, _ = fmt.Fprintln(output.Stdout(), result.DockerCompose)
			}
			if result.EnvVars != "" {
				_, _ = fmt.Fprintln(output.Stdout(), "Environment Variables:")
				_, _ = fmt.Fprintln(output.Stdout(), result.EnvVars)
			}
			return nil
		},
	}
	cmdutil.SetScope(cmdutil.ScopeDestination, cmd)
	return cmd
}
