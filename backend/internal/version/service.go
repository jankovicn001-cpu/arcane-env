package version

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/moby/moby/client"
	"golang.org/x/mod/semver"

	"kit.getarcane.app"
	"kit.getarcane.app/cache"
	"kit.getarcane.app/docker/compat"
	"kit.getarcane.app/docker/streams/agg"
	"kit.getarcane.app/sys/cgroup"

	"getarcane.dev/arcane/v3/buildables"
	"getarcane.dev/arcane/v3/internal/apns"
	"getarcane.dev/arcane/v3/internal/config"
	"getarcane.dev/arcane/v3/internal/docker"
	"getarcane.dev/arcane/v3/pkg/selfcontainer"
	"getarcane.dev/types/v3"
)

type (
	currentImageInfo struct {
		Tag    string
		Digest string
	}

	VersionService struct {
		dockerService *docker.DockerClientService
		currentImage  *cache.Cache[struct{}, currentImageInfo]
		version       string
		revision      string
	}

	VersionServiceParams struct {
		DockerService *docker.DockerClientService
		AppVersion    string
		Revision      string
	}
)

func NewVersionService(p VersionServiceParams) *VersionService {
	return &VersionService{
		version:       p.AppVersion,
		revision:      p.Revision,
		dockerService: p.DockerService,
		currentImage: &cache.Cache[struct{}, currentImageInfo]{
			Capacity: 1,
		},
	}
}

func (s *VersionService) normalizeVersion(ver string) string {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return "v0.0.0"
	}
	trimmed := strings.TrimPrefix(ver, "v")
	if trimmed == "" || trimmed[0] < '0' || trimmed[0] > '9' {
		return ver
	}
	ver = kit.EnsurePrefix(ver, "v")
	if !semver.IsValid(ver) {
		if idx := strings.IndexAny(ver, "-+"); idx > 0 {
			ver = ver[:idx]
		}
		parts := strings.Split(strings.TrimPrefix(ver, "v"), ".")
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
		ver = "v" + strings.Join(parts[:3], ".")
	}
	return ver
}

func (s *VersionService) GetAppVersionInfo(ctx context.Context) *types.VersionInfo {
	isSemver := semver.IsValid(kit.EnsurePrefix(s.version, "v"))
	// Semver versions (including prereleases like 2.4.0-next.1) display as v<version>.
	displayVersion := strings.TrimPrefix(strings.TrimSpace(s.version), "v")
	current := s.detectCurrentImageInfo(ctx)

	return &types.VersionInfo{
		CurrentVersion:   s.normalizeVersion(s.version),
		CurrentTag:       current.Tag,
		CurrentDigest:    current.Digest,
		DisplayVersion:   kit.Ternary(isSemver, "v"+displayVersion, displayVersion),
		Revision:         s.revision,
		ShortRevision:    config.ShortRevision(),
		GoVersion:        runtime.Version(),
		NodeVersion:      config.NodeVersion,
		SvelteKitVersion: config.SvelteKitVersion,
		EnabledFeatures:  append(kit.Unique(kit.TrimNonEmpty(strings.Split(strings.ToLower(buildables.EnabledFeatures), ","))), apns.FeatureName),
		BuildTime:        config.BuildTime,
		IsSemverVersion:  isSemver,
		IsStableVersion:  isSemver && semver.Prerelease(kit.EnsurePrefix(s.version, "v")) == "",
	}
}

func (s *VersionService) detectCurrentImageInfo(ctx context.Context) (detected currentImageInfo) {
	if cached, found, _ := s.currentImage.Get(struct{}{}); found {
		return cached
	}
	defer func() {
		if ctx.Err() != nil {
			return
		}
		if detected.Digest != "" {
			s.currentImage.Set(cache.Entry[struct{}, currentImageInfo]{
				Key:   struct{}{},
				Value: detected,
			})
		} else {
			s.currentImage.Set(cache.Entry[struct{}, currentImageInfo]{
				Key:   struct{}{},
				Value: detected,
				TTL:   time.Minute,
			})
		}
	}()

	dockerClient, err := s.dockerService.GetClient(ctx)
	if err != nil {
		slog.DebugContext(ctx, "detectCurrentImageInfo: failed to get docker client", "error", err)
		return currentImageInfo{}
	}

	containerId, err := cgroup.CurrentContainerID()
	if err != nil {
		// Fall back to the Arcane label when cgroup/hostname detection fails.
		slog.DebugContext(ctx, "detectCurrentImageInfo: cgroup detection failed, trying label fallback", "error", err)
		containerId = selfcontainer.FindArcaneContainerIDByLabel(ctx, dockerClient)
	}
	if containerId == "" {
		slog.DebugContext(ctx, "detectCurrentImageInfo: could not detect container ID")
		return currentImageInfo{}
	}
	slog.DebugContext(ctx, "detectCurrentImageInfo: detected container", "containerId", containerId)

	inspectResult, err := s.dockerService.Call(ctx, func(ctx context.Context, cli *client.Client) (client.ContainerInspectResult, error) {
		return compat.ContainerInspectWithCompatibility(ctx, cli, containerId, client.ContainerInspectOptions{})
	})
	if err != nil {
		slog.DebugContext(ctx, "detectCurrentImageInfo: failed to inspect container", "containerId", containerId, "error", err)
		return currentImageInfo{}
	}

	// The tag comes from the user-specified config image.
	tag := "latest"
	if named, parseErr := reference.ParseNormalizedNamed(kit.FromPtr(inspectResult.Container.Config).Image); parseErr == nil {
		if tagged, ok := named.(reference.Tagged); ok {
			tag = tagged.Tag()
		}
	}
	base := currentImageInfo{Tag: tag}

	imageID := inspectResult.Container.Image
	if imageID == "" {
		return base
	}
	imageInspect, err := s.dockerService.Call(ctx, func(ctx context.Context, cli *client.Client) (client.ImageInspectResult, error) {
		return cli.ImageInspect(ctx, imageID)
	})
	if err != nil {
		return base
	}
	for _, repoDigest := range imageInspect.RepoDigests {
		repoNamed, repoParseErr := reference.ParseNormalizedNamed(repoDigest)
		if repoParseErr != nil {
			continue
		}
		if digested, ok := repoNamed.(reference.Digested); ok {
			return currentImageInfo{Tag: base.Tag, Digest: string(digested.Digest())}
		}
	}
	return base
}

// RunStreamProducer sends one snapshot; the running version only changes on restart.
func (s *VersionService) RunStreamProducer(ctx context.Context, events chan<- types.VersionStreamEvent) {
	agg.Send(ctx, events, types.VersionStreamEvent{Type: "snapshot", Info: s.GetAppVersionInfo(ctx), Timestamp: time.Now()})
}
