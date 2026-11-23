package recovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"kit.getarcane.app"
	"kit.getarcane.app/docker"
	"kit.getarcane.app/docker/compat"
	"kit.getarcane.app/updater/labels"

	"getarcane.dev/arcane/v3/internal/config/schema"
	"getarcane.dev/arcane/v3/pkg/projects"
)

// recreateContainer replaces the Arcane container with one running image and the recovered environment.
func recreateContainer(ctx context.Context, dockerClient *client.Client, oldContainer container.InspectResponse, image string, environment map[string]string) error {
	originalName := strings.TrimPrefix(oldContainer.Name, "/")
	oldName := fmt.Sprintf("%s-old-%d", originalName, time.Now().UnixNano())

	if oldContainer.Config == nil {
		return fmt.Errorf("container %s inspection has no config; refusing to recreate", originalName)
	}

	containerConfig := *oldContainer.Config
	containerConfig.Image = image
	containerConfig.Labels = normalizeRecreatedArcaneLabels(containerConfig.Labels)
	containerConfig.Env = applyRecoveredEnvironment(containerConfig.Env, environment)

	containerConfig.Labels = refreshRecreatedContainerLabels(ctx, dockerClient, containerConfig.Labels, oldContainer.Image, image)

	hostConfig, sanitizedMemorySwappiness, engineInfo, err := compat.PrepareRecreateHostConfigForEngine(ctx, dockerClient, oldContainer.ID, oldContainer.HostConfig)
	if err != nil {
		return fmt.Errorf("prepare host config: %w", err)
	}
	if sanitizedMemorySwappiness {
		slog.InfoContext(ctx, "Stripped unsupported host config field for recreate",
			"container", originalName,
			"containerId", oldContainer.ID,
			"engine", engineInfo.Name,
			"cgroupVersion", engineInfo.CgroupVersion,
			"field", "memorySwappiness",
		)
	}

	// Pin every attached volume to its resolved name so the recreated container
	// keeps its data instead of receiving fresh anonymous volumes (#3953).
	if hostConfig == nil {
		hostConfig = &container.HostConfig{}
	}
	hostConfig.Binds, hostConfig.Mounts, err = docker.PreserveVolumeMounts(ctx, hostConfig.Binds, hostConfig.Mounts, oldContainer.Mounts)
	if err != nil {
		return fmt.Errorf("preserve volumes of container %s: %w", originalName, err)
	}

	nm := hostConfig.NetworkMode
	if nm.IsHost() || nm.IsContainer() {
		containerConfig.Hostname = ""
		containerConfig.Domainname = ""
	}

	if looksLikeContainerID(containerConfig.Hostname) {
		slog.DebugContext(ctx, "Clearing auto-generated hostname", "oldHostname", containerConfig.Hostname)
		containerConfig.Hostname = ""
	}

	if nm.IsContainer() {
		containerConfig.ExposedPorts = nil
		hostConfig.PortBindings = nil
		hostConfig.PublishAllPorts = false
	}

	var (
		apiVersion    string
		networkConfig *network.NetworkingConfig
	)
	if !nm.IsContainer() {
		apiVersion = compat.DetectDockerAPIVersion(ctx, dockerClient)
		if apiVersion != "" && !compat.IsDockerAPIVersionAtLeast(apiVersion, compat.NetworkScopedMacAddressMinAPIVersion) {
			slog.InfoContext(ctx, "daemon API does not support per-network mac-address on create; stripping endpoint mac addresses",
				"dockerApiVersion", apiVersion,
				"minimumRequiredApiVersion", compat.NetworkScopedMacAddressMinAPIVersion,
			)
		}

		var endpoints map[string]*network.EndpointSettings
		if oldContainer.NetworkSettings != nil {
			endpoints = oldContainer.NetworkSettings.Networks
		}

		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: compat.SanitizeContainerCreateEndpointSettingsForDockerAPI(endpoints, apiVersion),
		}
	}

	slog.InfoContext(ctx, "Renaming old container", "from", originalName, "to", oldName)
	if _, containerRenameErr := dockerClient.ContainerRename(ctx, oldContainer.ID, client.ContainerRenameOptions{NewName: oldName}); containerRenameErr != nil {
		return fmt.Errorf("rename old container: %w", containerRenameErr)
	}

	slog.InfoContext(ctx, "Stopping old container", "name", oldName)
	// Allow graceful shutdown and longer finite timeouts, but never wait forever.
	stopTimeout := max(40, kit.FromPtr(oldContainer.Config.StopTimeout))
	if _, containerStopErr := dockerClient.ContainerStop(ctx, oldContainer.ID, client.ContainerStopOptions{Timeout: &stopTimeout}); containerStopErr != nil {
		_, _ = dockerClient.ContainerRename(ctx, oldContainer.ID, client.ContainerRenameOptions{NewName: originalName})
		return fmt.Errorf("stop old container: %w", containerStopErr)
	}

	slog.InfoContext(ctx, "Creating new container", "name", originalName)
	resp, err := compat.ContainerCreateWithCompatibility(ctx, dockerClient, compat.CreateRequest{
		Options: client.ContainerCreateOptions{
			Config:           &containerConfig,
			HostConfig:       hostConfig,
			NetworkingConfig: networkConfig,
			Name:             originalName,
		},
		APIVersion: apiVersion,
	})
	if err != nil {
		_, _ = dockerClient.ContainerStart(ctx, oldContainer.ID, client.ContainerStartOptions{})
		_, _ = dockerClient.ContainerRename(ctx, oldContainer.ID, client.ContainerRenameOptions{NewName: originalName})
		return fmt.Errorf("create new container: %w", err)
	}

	slog.InfoContext(ctx, "Starting new container", "id", resp.ID[:12])
	if _, startErr := dockerClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); startErr != nil {
		err = fmt.Errorf("start new container: %w", startErr)
		if _, removeErr := dockerClient.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true}); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove new container: %w", removeErr))
		}
		if _, restartErr := dockerClient.ContainerStart(ctx, oldContainer.ID, client.ContainerStartOptions{}); restartErr != nil {
			err = errors.Join(err, fmt.Errorf("restart old container: %w", restartErr))
		}
		if _, renameErr := dockerClient.ContainerRename(ctx, oldContainer.ID, client.ContainerRenameOptions{NewName: originalName}); renameErr != nil {
			err = errors.Join(err, fmt.Errorf("restore old container name: %w", renameErr))
		}
		return err
	}

	time.Sleep(2 * time.Second)

	slog.InfoContext(ctx, "Removing old container", "id", oldContainer.ID[:12])
	if _, containerRemoveErr := dockerClient.ContainerRemove(ctx, oldContainer.ID, client.ContainerRemoveOptions{}); containerRemoveErr != nil {
		slog.WarnContext(ctx, "Failed to remove old container", "error", containerRemoveErr)
	}

	return nil
}

func isLegacyServerLabel(containerLabels map[string]string) bool {
	if containerLabels == nil {
		return false
	}
	for k, v := range containerLabels {
		if strings.EqualFold(k, labels.LabelArcaneLegacyServer) {
			return strings.EqualFold(strings.TrimSpace(v), "true")
		}
	}
	return false
}

func normalizeRecreatedArcaneLabels(containerLabels map[string]string) map[string]string {
	normalized := maps.Clone(containerLabels)
	if normalized == nil {
		return nil
	}
	if labels.IsArcaneContainer(containerLabels) || isLegacyServerLabel(containerLabels) {
		normalized[labels.LabelArcane] = "true"
	}
	if labels.IsArcaneAgentContainer(containerLabels) {
		normalized[labels.LabelArcaneAgent] = "true"
		normalized[labels.LabelArcane] = "true"
	}
	return normalized
}

func refreshRecreatedImageLabels(containerLabels, previousImageLabels, imageLabels map[string]string, imageDigest string) map[string]string {
	refreshed := maps.Clone(containerLabels)
	for key, value := range containerLabels {
		if !strings.HasPrefix(strings.ToLower(key), "org.opencontainers.image.") {
			continue
		}
		for previousKey, previousValue := range previousImageLabels {
			if strings.EqualFold(previousKey, key) && previousValue == value {
				delete(refreshed, key)
				break
			}
		}
	}

	for key, value := range imageLabels {
		if !strings.HasPrefix(strings.ToLower(key), "org.opencontainers.image.") {
			continue
		}
		preserved := false
		for existingKey := range refreshed {
			if strings.EqualFold(existingKey, key) {
				preserved = true
				break
			}
		}
		if preserved {
			continue
		}
		if refreshed == nil {
			refreshed = make(map[string]string)
		}
		refreshed[key] = value
	}

	projects.RefreshComposeImageLabel(refreshed, imageDigest)

	return refreshed
}

func refreshRecreatedContainerLabels(ctx context.Context, dockerClient *client.Client, containerLabels map[string]string, previousImage, targetImage string) map[string]string {
	imageInspect, imageInspectErr := dockerClient.ImageInspect(ctx, targetImage, client.ImageInspectWithManifests(true))
	if imageInspectErr != nil {
		imageInspect, imageInspectErr = dockerClient.ImageInspect(ctx, targetImage)
	}
	if imageInspectErr != nil {
		slog.WarnContext(ctx, "Could not inspect target image labels; preserving container OCI overrides",
			"image", targetImage,
			"error", imageInspectErr,
		)
		return maps.Clone(containerLabels)
	}

	imageLabels := map[string]string(nil)
	previousImageLabels := map[string]string(nil)
	if imageInspect.Config != nil {
		imageLabels = imageInspect.Config.Labels
		if hasOCIImageLabel(containerLabels) && strings.TrimSpace(previousImage) != "" {
			previousImageInspect, previousImageInspectErr := dockerClient.ImageInspect(ctx, previousImage)
			if previousImageInspectErr != nil {
				slog.WarnContext(ctx, "Could not inspect previous image labels; preserving existing OCI labels",
					"image", previousImage,
					"error", previousImageInspectErr,
				)
			} else if previousImageInspect.Config != nil {
				previousImageLabels = previousImageInspect.Config.Labels
			}
		}
	}
	return refreshRecreatedImageLabels(containerLabels, previousImageLabels, imageLabels, projects.ComposeImageDigest(imageInspect))
}

func hasOCIImageLabel(containerLabels map[string]string) bool {
	for key := range containerLabels {
		if strings.HasPrefix(strings.ToLower(key), "org.opencontainers.image.") {
			return true
		}
	}
	return false
}

func applyRecoveredEnvironment(current []string, recovered map[string]string) []string {
	renamed := schema.RenamedEnv()
	present := make(map[string]struct{}, len(current))
	for _, entry := range current {
		name, _, _ := strings.Cut(entry, "=")
		present[name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(recovered))
	updated := make([]string, 0, len(current)+len(recovered))
	for _, entry := range current {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			updated = append(updated, entry)
			continue
		}
		for _, suffix := range []string{"", "__FILE", "_FILE"} {
			base, ok := strings.CutSuffix(name, suffix)
			target, legacy := renamed[base]
			if !ok || !legacy {
				continue
			}
			if _, exists := present[target+suffix]; !exists {
				name = target + suffix
				entry = name + "=" + value
			}
			break
		}
		if baseName, ok := strings.CutSuffix(name, "__FILE"); ok {
			if _, localOk := recovered[baseName]; localOk {
				continue
			}
		}
		if baseName, ok := strings.CutSuffix(name, "_FILE"); ok {
			if _, localOk2 := recovered[baseName]; localOk2 {
				continue
			}
		}
		if replacement, ok := recovered[name]; ok {
			updated = append(updated, name+"="+replacement)
			seen[name] = struct{}{}
		} else {
			updated = append(updated, entry)
		}
	}
	for name, value := range recovered {
		if _, ok := seen[name]; !ok {
			updated = append(updated, name+"="+value)
		}
	}
	return updated
}

func looksLikeContainerID(s string) bool {
	if len(s) != 12 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
