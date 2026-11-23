package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/arcane/v3/pkg/selfcontainer"
	"getarcane.dev/types/v3"
)

func TestParseArcaneComposeMetadata_InterpolationAndAnchor(t *testing.T) {
	tempDir := t.TempDir()

	envContent := "ARCANE_TEST_DOMAIN=example.com\nARCANE_TEST_ICONS_CDN=https://cdn.jsdelivr.net/gh/homarr-labs\n"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0o600))

	composeContent := `services:
  app:
    image: nginx:alpine
x-arcane-icon-light: &arcane-icon "${ARCANE_TEST_ICONS_CDN}/webp/raspberry-pi.webp"
x-arcane:
  icon-light: *arcane-icon
  icon-dark: *arcane-icon
  urls:
    - https://www.${ARCANE_TEST_DOMAIN}
`

	composePath := filepath.Join(tempDir, "compose.yaml")
	require.NoError(t, os.WriteFile(composePath, []byte(composeContent), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), composePath, tempDir, false)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/homarr-labs/webp/raspberry-pi.webp", meta.ProjectIcon.Light)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/homarr-labs/webp/raspberry-pi.webp", meta.ProjectIcon.Dark)
	require.Equal(t, []string{"https://www.example.com"}, meta.ProjectURLS)
}

func TestParseArcaneComposeMetadata_ServiceLabelIconsOutrankProjectIcon(t *testing.T) {
	tempDir := t.TempDir()

	composeContent := `x-arcane:
  icon-light: umami
  icon-dark: umami
services:
  umami:
    image: ghcr.io/umami-software/umami:latest
  db:
    image: postgres:16
    labels:
      com.getarcaneapp.arcane.icon-light: postgres
      com.getarcaneapp.arcane.icon-dark: postgres
`
	composePath := filepath.Join(tempDir, "compose.yaml")
	require.NoError(t, os.WriteFile(composePath, []byte(composeContent), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), composePath, tempDir, false)
	require.NoError(t, err)
	require.Equal(t, IconSet{Light: "postgres", Dark: "postgres"}, meta.ServiceIconSets["db"])
	require.NotContains(t, meta.ServiceIconSets, "umami")
}

func TestParseArcaneComposeMetadata_IncludeSupport(t *testing.T) {
	tempDir := t.TempDir()

	composeContent := `include:
  - meta.yaml
services:
  app:
    image: nginx:alpine
`
	composePath := filepath.Join(tempDir, "compose.yaml")
	require.NoError(t, os.WriteFile(composePath, []byte(composeContent), 0o600))

	metaContent := `x-arcane:
  icon-light: https://example.com/icon-light.png
  icon-dark: https://example.com/icon-dark.png
  urls:
    - https://example.com/docs
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "meta.yaml"), []byte(metaContent), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), composePath, tempDir, false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/icon-light.png", meta.ProjectIcon.Light)
	require.Equal(t, "https://example.com/icon-dark.png", meta.ProjectIcon.Dark)
	require.Equal(t, []string{"https://example.com/docs"}, meta.ProjectURLS)
}

func TestParseArcaneComposeMetadata_TagNamesColorsInterpolationAndIncludes(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".env"), []byte("PROJECT_KIND=Database\nPROJECT_COLOR=Purple\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "compose.yaml"), []byte(`include:
  - included.yaml
x-arcane:
  tags:
    - name: ${PROJECT_KIND}
      color: ${PROJECT_COLOR}
services:
  app:
    image: nginx:alpine
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "included.yaml"), []byte(`x-arcane:
  tags:
    - name: maintenance-window
      color: orange
    - name: DATABASE
      color: red
    - name: invalid-color
      color: chartreuse
    - name: missing-color
services: {}
`), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), filepath.Join(tempDir, "compose.yaml"), tempDir, false)
	require.NoError(t, err)
	require.Equal(t, []types.ProjectTagOption{
		{Name: "database", Color: types.ProjectTagColorPurple},
		{Name: "maintenance-window", Color: types.ProjectTagColorOrange},
	}, meta.ProjectTags)
}

func TestParseArcaneComposeMetadata_LoadsGlobalEnvForIncludedMetadata(t *testing.T) {
	projectsRoot := t.TempDir()
	projectDir := filepath.Join(projectsRoot, "demo")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectsRoot, GlobalEnvFileName),
		[]byte("ICON_CDN_URL=https://cdn.jsdelivr.net/gh/selfhst/icons@main\n"),
		0o600,
	))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "compose.yaml"), []byte(`include:
  - metadata.yaml
services:
  watchtower:
    image: nickfedor/watchtower:latest
`), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "metadata.yaml"), []byte(`x-watchtower-icon-light: &watchtower-icon "${ICON_CDN_URL:+${ICON_CDN_URL}/svg/watchtower.svg}"
x-arcane:
  icon-light: *watchtower-icon
  icon-dark: *watchtower-icon
`), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), filepath.Join(projectDir, "compose.yaml"), projectsRoot, false)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/selfhst/icons@main/svg/watchtower.svg", meta.ProjectIcon.Light)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/selfhst/icons@main/svg/watchtower.svg", meta.ProjectIcon.Dark)
}

func TestParseArcaneComposeMetadata_LoadsGlobalEnvForNestedProjects(t *testing.T) {
	projectsRoot := t.TempDir()
	projectDir := filepath.Join(projectsRoot, "group", "demo")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectsRoot, GlobalEnvFileName),
		[]byte("ICON_CDN_URL=https://cdn.jsdelivr.net/gh/selfhst/icons@main\n"),
		0o600,
	))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "compose.yaml"), []byte(`include:
  - metadata.yaml
services:
  watchtower:
    image: nickfedor/watchtower:latest
`), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "metadata.yaml"), []byte(`x-watchtower-icon-light: &watchtower-icon "${ICON_CDN_URL:+${ICON_CDN_URL}/svg/watchtower.svg}"
x-arcane:
  icon-light: *watchtower-icon
  icon-dark: *watchtower-icon
`), 0o600))

	meta, err := ParseArcaneComposeMetadata(t.Context(), filepath.Join(projectDir, "compose.yaml"), projectsRoot, false)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/selfhst/icons@main/svg/watchtower.svg", meta.ProjectIcon.Light)
	require.Equal(t, "https://cdn.jsdelivr.net/gh/selfhst/icons@main/svg/watchtower.svg", meta.ProjectIcon.Dark)
}

func TestNormalizeProjectTags(t *testing.T) {
	tags, err := NormalizeProjectTags([]string{" Database ", "DATABASE", "maintenance-window"})
	require.NoError(t, err)
	require.Equal(t, []string{"database", "maintenance-window"}, tags)

	for _, invalid := range []string{"", "bad,tag", "bad\ntag", strings.Repeat("a", ProjectTagMaxLength+1)} {
		_, normalizeProjectTagErr := NormalizeProjectTag(invalid)
		require.Error(t, normalizeProjectTagErr, invalid)
	}

	tooMany := make([]string, ProjectTagsPerSourceLimit+1)
	for index := range tooMany {
		tooMany[index] = "tag-" + strings.Repeat("x", index/10) + string(rune('a'+index%10))
	}
	_, err = NormalizeProjectTags(tooMany)
	require.Error(t, err)
}

func TestHiddenMetadataInterpolationAndIncludes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("HIDE_MAIN=true\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`include:
  - included.yaml
x-arcane:
  hidden: "${HIDE_MAIN}"
services:
  main:
    image: alpine:3.20.0
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "included.yaml"), []byte(`services:
  child:
    image: alpine:3.20.0
    x-arcane:
      hidden: false
`), 0o600))
	project, err := LoadComposeProject(t.Context(), LoadOptions{
		ComposeFile:       filepath.Join(dir, "compose.yaml"),
		ProjectName:       "metadata",
		ProjectsDirectory: dir,
	})
	require.NoError(t, err)
	require.Equal(t, "true", project.Services["main"].Labels[selfcontainer.HiddenResourceLabel])
	require.Equal(t, "false", project.Services["child"].Labels[selfcontainer.HiddenResourceLabel])
}
