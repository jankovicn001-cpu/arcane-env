package details

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/moby/moby/api/types/container"

	"kit.getarcane.app"
	"kit.getarcane.app/docker"
	"kit.getarcane.app/sys/cgroup"
	"kit.getarcane.app/updater/labels"

	"getarcane.dev/arcane/v3/internal/database"
	dockerInternal "getarcane.dev/arcane/v3/internal/docker"
	"getarcane.dev/arcane/v3/internal/settings"
	"getarcane.dev/arcane/v3/pkg/iconcatalog"
	"getarcane.dev/arcane/v3/pkg/projects"
	"getarcane.dev/types/v3"
)

type (
	Service struct {
		db              *database.DB
		dockerService   *dockerInternal.DockerClientService
		settingsService *settings.SettingsService
	}
)

func New(db *database.DB, dockerService *dockerInternal.DockerClientService, settingsService *settings.SettingsService) *Service {
	return &Service{
		db:              db,
		dockerService:   dockerService,
		settingsService: settingsService,
	}
}

func (s *Service) ComposeContainers(ctx context.Context) ([]container.Summary, error) {
	containers, err := s.dockerService.ListContainers(ctx, false)
	if err != nil {
		return nil, err
	}
	composeContainers := make([]container.Summary, 0, len(containers))
	for _, c := range containers {
		if _, compose := c.Labels[api.ProjectLabel]; compose {
			composeContainers = append(composeContainers, c)
		}
	}
	return composeContainers, nil
}

func RuntimeProjectName(fallback string, runtimeContainers []container.Summary) string {
	if len(runtimeContainers) == 0 {
		return fallback
	}
	return cmp.Or(docker.ComposeProjectLabel(runtimeContainers[0].Labels), fallback)
}

func (s *Service) ComposeServices(
	ctx context.Context,
	composeProject *composetypes.Project,
	runtimeContainers []container.Summary,
	meta projects.ArcaneComposeMetadata,
	catalog string,
) ([]types.ProjectRuntimeService, error) {
	runtimeName := RuntimeProjectName(composeProject.Name, runtimeContainers)
	containers, err := projects.ComposePs(ctx, types.ComposePsOptions{
		Project:    &composetypes.Project{Name: runtimeName},
		DockerHost: s.dockerService.DockerHost(),
		All:        true,
	})
	if err != nil {
		slog.ErrorContext(ctx, "compose ps error", "projectName", runtimeName, "error", err)
		return nil, fmt.Errorf("failed to get compose services status: %w", err)
	}
	imageIDs := make(map[string]string, len(runtimeContainers))
	for _, c := range runtimeContainers {
		if c.ImageID != "" {
			imageIDs[c.ID] = c.ImageID
		}
	}
	currentContainerID, currentContainerErr := cgroup.CurrentContainerID()

	have := map[string]bool{}
	var services []types.ProjectRuntimeService
	for _, c := range containers {
		if c.Labels[projects.JobRunLabel] != "" || c.Labels[api.OneoffLabel] == "True" {
			continue
		}
		resolvedIcon := iconcatalog.Resolve(catalog, cmp.Or(
			projects.FindArcaneIconSet(c.Labels),
			meta.ServiceIconSets[c.Service],
			meta.ProjectIcon,
		))
		services = append(services, types.ProjectRuntimeService{
			Name:          c.Service,
			Image:         c.Image,
			Status:        string(c.State),
			ContainerID:   c.ID,
			ContainerName: c.Name,
			Ports:         projects.FormatPorts(c.Publishers),
			Health:        kit.Ternary(c.Health != "", new(string(c.Health)), nil),
			IconLightURL:  resolvedIcon.IconLightURL,
			IconDarkURL:   resolvedIcon.IconDarkURL,
			ServiceConfig: func() *composetypes.ServiceConfig {
				if cfg, ok := composeProject.Services[c.Service]; ok {
					return &cfg
				}
				return nil
			}(),
			ContainerLabels: c.Labels,
			RedeployDisabled: labels.ShouldDisableArcaneServerRedeploy(labels.RedeployCheck{
				CurrentErr:         currentContainerErr,
				Labels:             c.Labels,
				ContainerID:        c.ID,
				CurrentContainerID: currentContainerID,
			}),
			ImageID: imageIDs[c.ID],
		})
		have[c.Service] = true
	}

	for _, svc := range composeProject.Services {
		if !have[svc.Name] {
			resolvedIcon := iconcatalog.Resolve(catalog, cmp.Or(
				meta.ServiceIconSets[svc.Name],
				meta.ProjectIcon,
			))
			services = append(services, types.ProjectRuntimeService{
				Name:          svc.Name,
				Image:         svc.Image,
				Status:        "stopped",
				Ports:         []string{},
				IconLightURL:  resolvedIcon.IconLightURL,
				IconDarkURL:   resolvedIcon.IconDarkURL,
				ServiceConfig: new(svc),
			})
		}
	}

	return services, nil
}

func (s *Service) IncludeFiles(ctx context.Context, composeFile string) []types.ProjectIncludeFile {
	if strings.TrimSpace(composeFile) == "" {
		return nil
	}

	// Loaded so include paths with ${VAR} references are expanded.
	cfg := s.settingsService.GetSettingsOrDefaults(ctx)
	projectsDirectory, _ := projects.GetProjectsDirectory(ctx, strings.TrimSpace(cfg.ProjectsDirectory.Value))
	envMap, _, _ := projects.NewEnvLoader(projectsDirectory, filepath.Dir(composeFile), kit.ParseOrDefault(cfg.AutoInjectEnv.Value, false, strconv.ParseBool)).LoadEnvironment(ctx)

	includes, parseErr := projects.ParseIncludes(ctx, composeFile, envMap, false)
	if parseErr != nil {
		slog.WarnContext(ctx, "Failed to parse includes", "error", parseErr, "path", composeFile)
		return nil
	}
	var includeFiles []types.ProjectIncludeFile
	for _, inc := range includes {
		includeFiles = append(includeFiles, types.ProjectIncludeFile{
			Path:         inc.Path,
			RelativePath: inc.RelativePath,
		})
	}
	return includeFiles
}

func Logs(ctx context.Context, dockerHost, projectName string, logsChan chan<- string, opts types.LogStreamOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	done := make(chan error, 2)

	go func() {
		// Closing the read half unblocks a pending pw.Write in ComposeLogs after an abandoned tail.
		defer cancel()
		defer func() { _ = pr.CloseWithError(io.ErrClosedPipe) }()

		reader := bufio.NewReaderSize(pr, 64*1024)
		var line []byte
		for {
			chunk, isPrefix, readErr := reader.ReadLine()
			if readErr != nil {
				done <- readErr
				return
			}
			line = append(line, chunk...)
			if isPrefix && len(line) < 1024*1024 {
				continue
			}
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case logsChan <- string(line):
			}
			line = line[:0]
		}
	}()

	go func() {
		composeLogsErr := projects.ComposeLogs(ctx, dockerHost, projects.NormalizeProjectName(projectName), pw, opts)
		_ = pw.Close()
		done <- composeLogsErr
	}()

	for _, e := range []error{<-done, <-done} {
		if e != nil && !errors.Is(e, io.EOF) && !errors.Is(e, context.Canceled) {
			return e
		}
	}
	return nil
}

func ResolveDetailsOverride(ctx context.Context, projectPath, overrideContent string, composeSelection []string) (fileName, content string) {
	overridePath := projects.DetectComposeOverrideFile(ctx, projectPath)
	if len(composeSelection) > 0 {
		if overridePath == "" {
			return "", ""
		}
		// Full paths keep a same-named subdirectory file from matching.
		absOverride, err := filepath.Abs(filepath.Clean(overridePath))
		if err != nil || !slices.Contains(composeSelection, absOverride) {
			return "", ""
		}
	}
	return kit.Ternary(overridePath != "", filepath.Base(overridePath), ""), overrideContent
}

func ComposeSelectionRelativePaths(projectPath string, selection []string) []string {
	if len(selection) <= 1 {
		return nil
	}
	rels := make([]string, 0, len(selection))
	for _, f := range selection {
		rel, err := filepath.Rel(projectPath, f)
		rels = append(rels, kit.Ternary(err == nil, rel, filepath.Base(f)))
	}
	return rels
}
