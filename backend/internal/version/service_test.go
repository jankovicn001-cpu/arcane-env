package version

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/libtnb/sqlite"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"kit.getarcane.app/docker/streams/bus"
	"kit.getarcane.app/updater/labels"

	"getarcane.dev/arcane/v3/internal/config"
	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/arcane/v3/internal/docker"
	"getarcane.dev/arcane/v3/internal/settings"
)

func TestVersionService_GetAppVersionInfoDisplaysSemverNextVersion(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	svc := NewVersionService(VersionServiceParams{
		AppVersion:    "2.4.0-next.2",
		Revision:      "2c3e44a10ddda540d7e19fc2a876c931fc33a426",
		DockerService: newVersionDockerService(t, server),
	})

	info := svc.GetAppVersionInfo(t.Context())

	require.NotNil(t, info)
	assert.Equal(t, "v2.4.0-next.2", info.DisplayVersion)
	assert.True(t, info.IsSemverVersion)
	assert.False(t, info.IsStableVersion)
}

func TestVersionService_GetAppVersionInfoPreservesDevVersion(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	svc := NewVersionService(VersionServiceParams{
		AppVersion:    "dev",
		Revision:      "unknown",
		DockerService: newVersionDockerService(t, server),
	})

	info := svc.GetAppVersionInfo(t.Context())

	require.NotNil(t, info)
	assert.Equal(t, "dev", info.CurrentVersion)
	assert.Equal(t, "dev", info.DisplayVersion)
	assert.False(t, info.IsSemverVersion)
}

func TestVersionService_GetAppVersionInfoReportsRunningImage(t *testing.T) {
	const (
		containerID = "arcane-container-1234567890"
		imageID     = "sha256:arcane-image"
		repo        = "ghcr.io/getarcaneapp/arcane"
		tag         = "v2.4.0"
	)
	currentDigest := digest.FromString("current-arcane").String()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			assert.NoError(t, json.MarshalWrite(w, []container.Summary{
				{ID: containerID, State: container.StateRunning, Labels: map[string]string{labels.LabelArcane: "true"}},
			}))
		case strings.Contains(r.URL.Path, "/containers/") && strings.HasSuffix(r.URL.Path, "/json"):
			assert.NoError(t, json.MarshalWrite(w, container.InspectResponse{
				ID:     containerID,
				Image:  imageID,
				Config: &container.Config{Image: repo + ":" + tag, Labels: map[string]string{labels.LabelArcane: "true"}},
			}))
		case strings.Contains(r.URL.Path, "/images/") && strings.HasSuffix(r.URL.Path, "/json"):
			assert.NoError(t, json.MarshalWrite(w, image.InspectResponse{
				ID:          imageID,
				RepoTags:    []string{repo + ":" + tag},
				RepoDigests: []string{repo + "@" + currentDigest},
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	svc := NewVersionService(VersionServiceParams{
		AppVersion:    "2.4.0",
		Revision:      "2c3e44a10ddda540d7e19fc2a876c931fc33a426",
		DockerService: newVersionDockerService(t, server),
	})

	info := svc.GetAppVersionInfo(t.Context())

	require.NotNil(t, info)
	assert.Equal(t, "v2.4.0", info.CurrentVersion)
	assert.Equal(t, tag, info.CurrentTag)
	assert.Equal(t, currentDigest, info.CurrentDigest)
	assert.True(t, info.IsStableVersion)
}

func newVersionDockerService(t *testing.T, server *httptest.Server) *docker.DockerClientService {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&settings.SettingVariable{}))
	settingsService, err := settings.NewSettingsService(settings.SettingsServiceParams{
		Ctx:    t.Context(),
		DB:     &database.DB{DB: gdb},
		Config: &config.Config{},
		Env:    settings.Env{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, settingsService.Stop(context.WithoutCancel(t.Context()))) })

	cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.41"), client.WithHTTPClient(server.Client()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })
	return docker.NewDockerClientService(t.Context(), nil, settingsService, bus.Config{}).WithClient(cli)
}
