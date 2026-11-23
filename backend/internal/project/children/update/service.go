package update

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"github.com/samber/mo"

	"kit.getarcane.app"
	"kit.getarcane.app/acfs"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/arcane/v3/internal/docker"
	"getarcane.dev/arcane/v3/internal/kv"
	"getarcane.dev/arcane/v3/pkg/projects"
	"getarcane.dev/types/v3"
)

type (
	// Project rows stay with the parent, which supplies their state through callbacks.
	Service struct {
		kv            *kv.KVService
		dockerService *docker.DockerClientService
		projectState  func(ctx context.Context, projectID string) (name, path string, found bool, err error)
		restoreState  func(ctx context.Context, journal *types.ProjectRenameJournal) error
	}

	Params struct {
		KVService     *kv.KVService
		DockerService *docker.DockerClientService
		ProjectState  func(ctx context.Context, projectID string) (name, path string, found bool, err error)
		RestoreState  func(ctx context.Context, journal *types.ProjectRenameJournal) error
	}

	SyncState struct {
		SkipDiscoveredPaths map[string]struct{}
		ProtectSeenPaths    map[string]struct{}
	}

	RenameRequest struct {
		Migration         types.VolumeMigration
		OldDirName        *string
		Name              *string
		ProjectID         string
		OldName           string
		OldPath           string
		ProjectsDirectory string
	}
)

func New(p Params) *Service {
	return &Service{
		kv:            p.KVService,
		dockerService: p.DockerService,
		projectState:  p.ProjectState,
		restoreState:  p.RestoreState,
	}
}

func (s *Service) SyncState(ctx context.Context) SyncState {
	state := SyncState{
		SkipDiscoveredPaths: make(map[string]struct{}),
		ProtectSeenPaths:    make(map[string]struct{}),
	}
	entries, err := s.kv.ListByPrefix(ctx, types.ProjectRenameJournalKeyPrefix)
	if err != nil {
		slog.WarnContext(ctx, "failed to list project rename journals during filesystem sync", "error", err)
		return state
	}

	for _, entry := range entries {
		var journal types.ProjectRenameJournal
		if unmarshalErr := json.Unmarshal([]byte(entry.Value), &journal); unmarshalErr != nil {
			slog.WarnContext(ctx, "failed to decode project rename journal during filesystem sync", "key", entry.Key, "error", unmarshalErr)
			continue
		}
		if !projects.RenameJournalFilesystemSyncPending(journal.Phase) {
			continue
		}
		if oldPath := strings.TrimSpace(journal.OldPath); oldPath != "" {
			state.ProtectSeenPaths[filepath.Clean(oldPath)] = struct{}{}
		}
		if newPath := strings.TrimSpace(journal.NewPath); newPath != "" {
			state.SkipDiscoveredPaths[filepath.Clean(newPath)] = struct{}{}
		}
	}

	return state
}

func (s *Service) Prepare(req RenameRequest) *types.ProjectRenameJournal {
	newName := strings.TrimSpace(kit.FromPtr(req.Name))
	if newName == "" || req.OldName == newName {
		return nil
	}

	newDirName := strings.TrimSpace(projects.SanitizeProjectName(newName))
	if newDirName == "" || strings.Trim(newDirName, "_") == "" {
		return nil
	}

	return &types.ProjectRenameJournal{
		ProjectID:  req.ProjectID,
		OldName:    req.OldName,
		NewName:    newName,
		OldPath:    filepath.Clean(req.OldPath),
		NewPath:    filepath.Clean(filepath.Join(req.ProjectsDirectory, newDirName)),
		OldDirName: mo.PointerToOption(req.OldDirName).ToPointer(),
		NewDirName: newDirName,
		Phase:      types.ProjectRenameJournalPhaseStarted,
		Volumes: func() []types.VolumeJournalVolume {
			if source, ok := req.Migration.(types.VolumeJournalSource); ok {
				return source.JournalVolumes()
			}
			return nil
		}(),
	}
}

func (s *Service) WriteJournal(ctx context.Context, journal *types.ProjectRenameJournal, phase string) error {
	if journal == nil {
		return nil
	}
	*journal = types.ProjectRenameJournal{ //exhaustruct:enforce
		ProjectID:  journal.ProjectID,
		OldName:    journal.OldName,
		NewName:    journal.NewName,
		OldPath:    journal.OldPath,
		NewPath:    journal.NewPath,
		OldDirName: journal.OldDirName,
		NewDirName: journal.NewDirName,
		Phase:      phase,
		Volumes:    journal.Volumes,
		UpdatedAt:  time.Now().UTC(),
	}

	payload, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("marshal project rename journal: %w", err)
	}

	if setErr := s.kv.Set(ctx, types.ProjectRenameJournalKeyPrefix+journal.ProjectID, string(payload)); setErr != nil {
		return fmt.Errorf("write project rename journal: %w", setErr)
	}
	return nil
}

func (s *Service) clearRollbackCleanup(ctx context.Context, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return nil
	}
	return s.kv.Delete(ctx, types.ProjectRenameRollbackCleanupKeyPrefix+projectID)
}

func (s *Service) RecoverAll(ctx context.Context) error {
	entries, err := s.kv.ListByPrefix(ctx, types.ProjectRenameJournalKeyPrefix)
	if err != nil {
		return err
	}

	var recoverErr error
	for _, entry := range entries {
		var journal types.ProjectRenameJournal
		if unmarshalErr := json.Unmarshal([]byte(entry.Value), &journal); unmarshalErr != nil {
			recoverErr = errors.Join(recoverErr, fmt.Errorf("decode project rename journal %s: %w", entry.Key, unmarshalErr))
			continue
		}
		if strings.TrimSpace(journal.ProjectID) == "" {
			continue
		}
		name, path, found, stateErr := s.projectState(ctx, journal.ProjectID)
		if stateErr != nil {
			recoverErr = errors.Join(recoverErr, fmt.Errorf("recover project rename journal %s: load project for rename recovery: %w", entry.Key, stateErr))
			continue
		}
		if journalErr := projects.RecoverRenameJournal(
			ctx,
			&journal,
			found && (name == journal.NewName || filepath.Clean(path) == filepath.Clean(journal.NewPath)),
			s.Operations(),
		); journalErr != nil {
			recoverErr = errors.Join(recoverErr, fmt.Errorf("recover project rename journal %s: %w", entry.Key, journalErr))
		}
	}

	cleanups, err := s.kv.ListByPrefix(ctx, types.ProjectRenameRollbackCleanupKeyPrefix)
	if err != nil {
		return errors.Join(recoverErr, err)
	}
	for _, entry := range cleanups {
		var cleanup types.ProjectRenameRollbackCleanup
		if unmarshalErr := json.Unmarshal([]byte(entry.Value), &cleanup); unmarshalErr != nil {
			recoverErr = errors.Join(recoverErr, fmt.Errorf("decode project rename rollback cleanup %s: %w", entry.Key, unmarshalErr))
			continue
		}
		if cleanupErr := func() error {
			if strings.TrimSpace(cleanup.ProjectID) == "" {
				return nil
			}
			if len(cleanup.Volumes) == 0 {
				return s.clearRollbackCleanup(ctx, cleanup.ProjectID)
			}
			name, path, found, stateErr := s.projectState(ctx, cleanup.ProjectID)
			if stateErr != nil {
				return fmt.Errorf("load project for rename rollback cleanup: %w", stateErr)
			}
			if !found {
				slog.WarnContext(ctx, "clearing project rename rollback cleanup because project no longer exists", "projectId", cleanup.ProjectID)
				return s.clearRollbackCleanup(ctx, cleanup.ProjectID)
			}
			if name != cleanup.OldName || filepath.Clean(path) != filepath.Clean(cleanup.OldPath) {
				slog.WarnContext(ctx, "clearing project rename rollback cleanup because project state changed", "projectId", cleanup.ProjectID, "projectName", name, "projectPath", path)
				return s.clearRollbackCleanup(ctx, cleanup.ProjectID)
			}
			return projects.CleanupRenameRollbackTargets(ctx, &cleanup, s.Operations())
		}(); cleanupErr != nil {
			recoverErr = errors.Join(recoverErr, fmt.Errorf("recover project rename rollback cleanup %s: %w", entry.Key, cleanupErr))
		}
	}
	return recoverErr
}

func (s *Service) RecoverProject(ctx context.Context, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return nil
	}

	raw, ok, err := s.kv.Get(ctx, types.ProjectRenameJournalKeyPrefix+projectID)
	if err != nil || !ok {
		return err
	}

	var journal types.ProjectRenameJournal
	if unmarshalErr := json.Unmarshal([]byte(raw), &journal); unmarshalErr != nil {
		return fmt.Errorf("decode project rename journal: %w", unmarshalErr)
	}
	if strings.TrimSpace(journal.ProjectID) == "" {
		return nil
	}
	name, path, found, stateErr := s.projectState(ctx, journal.ProjectID)
	if stateErr != nil {
		return fmt.Errorf("load project for rename recovery: %w", stateErr)
	}
	return projects.RecoverRenameJournal(ctx, &journal, found && (name == journal.NewName || filepath.Clean(path) == filepath.Clean(journal.NewPath)), s.Operations())
}

func (s *Service) RollbackFailedUpdate(
	ctx context.Context,
	projectID, projectsDirectory, originalPath, currentPath string,
	backup *projects.ProjectUpdateBackup,
	journalActive bool,
	err error,
) error {
	renameErr := func() error {
		switch {
		case currentPath == originalPath:
			return nil
		case filepath.Dir(originalPath) == filepath.Dir(currentPath):
			return acfs.Root{Path: filepath.Dir(originalPath)}.Rename(context.WithoutCancel(ctx), "/"+filepath.Base(currentPath), "/"+filepath.Base(originalPath))
		}
		return acfs.Host{}.Rename(context.WithoutCancel(ctx), currentPath, originalPath)
	}()
	if renameErr != nil {
		slog.WarnContext(ctx, "failed to rollback project directory rename", "from", currentPath, "to", originalPath, "error", renameErr)
	}
	if backup != nil {
		if restoreErr := projects.RestoreProjectDirectoryBackup(ctx, projectsDirectory, kit.Ternary(renameErr == nil, originalPath, currentPath), backup); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to restore project files after update failure: %w", restoreErr))
		}
	}
	if journalActive {
		if recoverErr := s.RecoverProject(ctx, projectID); recoverErr != nil {
			err = errors.Join(err, fmt.Errorf("project rename recovery failed: %w", recoverErr))
		}
	}
	return err
}

func (s *Service) Operations() types.ProjectRenameRecoveryOperations {
	return types.ProjectRenameRecoveryOperations{
		Docker: func(ctx context.Context, dockerRequired bool) (*client.Client, error) {
			if !dockerRequired {
				return nil, nil
			}
			dockerClient, err := s.dockerService.GetClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to Docker: %w", err)
			}
			return dockerClient, nil
		},
		WriteJournal: s.WriteJournal,
		ClearJournal: func(ctx context.Context, projectID string) error {
			if strings.TrimSpace(projectID) == "" {
				return nil
			}
			return s.kv.Delete(ctx, types.ProjectRenameJournalKeyPrefix+projectID)
		},
		ClearRollbackCleanup: s.clearRollbackCleanup,
		WriteRollbackCleanup: func(ctx context.Context, journal *types.ProjectRenameJournal) error {
			if strings.TrimSpace(kit.FromPtr(journal).ProjectID) == "" || len(journal.Volumes) == 0 {
				return nil
			}
			payload, err := json.Marshal(types.ProjectRenameRollbackCleanup{
				ProjectID: journal.ProjectID,
				OldName:   journal.OldName,
				OldPath:   filepath.Clean(journal.OldPath),
				NewName:   journal.NewName,
				NewPath:   filepath.Clean(journal.NewPath),
				Volumes:   journal.Volumes,
				UpdatedAt: time.Now().UTC(),
			})
			if err != nil {
				return fmt.Errorf("marshal project rename rollback cleanup: %w", err)
			}
			if setErr := s.kv.Set(ctx, types.ProjectRenameRollbackCleanupKeyPrefix+journal.ProjectID, string(payload)); setErr != nil {
				return fmt.Errorf("write project rename rollback cleanup: %w", setErr)
			}
			return nil
		},
		RestoreState: s.restoreState,
	}
}

func EnsureProjectEnvReadable(ctx context.Context, projectsDirectory, projectPath string) error {
	cfgErr := projects.CheckProjectEnvAccess(ctx, projectsDirectory, projectPath)
	if !kit.FromPtr(cfgErr).BlocksOperations {
		return nil
	}
	return common.Classify(
		common.ErrProjectEnvUnreadable,
		fmt.Errorf(
			"%s is not readable by the runtime user (uid %d, gid %d); fix its ownership/read permission or set PUID/PGID to a user that can read it",
			cfgErr.Path,
			cfgErr.UID,
			cfgErr.GID,
		),
	)
}
