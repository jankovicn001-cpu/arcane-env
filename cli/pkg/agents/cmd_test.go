package agents_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"getarcane.dev/cli/v3/internal/testutil"
)

func TestEnvironmentsCreatePrintsEnrollmentTokenAndInstallCommands(t *testing.T) {
	var createBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/environment-groups":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"grp-1","name":"Prod"},{"id":"grp-2","name":"Staging"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents":
			assert.NoError(t, json.UnmarshalRead(r.Body, &createBody))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"env-1","name":"Agent One","apiUrl":"agent://agent-one","status":"pending","enabled":true,` +
				`"groupId":"grp-1","group":{"id":"grp-1","name":"Prod","description":""}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/env-1/deployment":
			_, _ = w.Write([]byte(`{"success":true,"data":{"enrollmentToken":"arc_etkn_secret","dockerRun":"docker run arcane-agent","dockerCompose":"services:\n  arcane-agent: {}"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
		}
	}))
	defer srv.Close()

	out, errOut, err := testutil.Execute(t, []string{"--config", testutil.WriteConfig(t, srv.URL), "agents", "create", "--name", "Agent One", "--environment", "prod"})
	require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)

	require.Equal(t, map[string]any{"name": "Agent One", "groupId": "grp-1", "enabled": true}, createBody, "environment names resolve case-insensitively")
	require.Contains(t, out, "Prod")
	require.Contains(t, out, "arc_etkn_secret")
	require.Contains(t, out, "docker run arcane-agent")
	require.Contains(t, out, "arcane-agent: {}")

	createBody = nil
	out, errOut, err = testutil.Execute(t, []string{"--config", testutil.WriteConfig(t, srv.URL), "agents", "create", "--name", "Agent One"})
	require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
	require.Equal(t, map[string]any{"name": "Agent One", "enabled": true}, createBody, "agents without --environment start unassigned")

	_, _, err = testutil.Execute(t, []string{"--config", testutil.WriteConfig(t, srv.URL), "agents", "create", "--name", "Agent One", "--environment", "missing"})
	require.ErrorContains(t, err, `environment "missing" not found`)
}

func TestEnvironmentsUpdateSendsGroupOnlyWhenEnvironmentFlagIsPassed(t *testing.T) {
	var updateBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/environment-groups/grp-2":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"grp-2","name":"Staging"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/agents/env-1":
			updateBody = nil
			assert.NoError(t, json.UnmarshalRead(r.Body, &updateBody))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"env-1","name":"Agent One","apiUrl":"agent://agent-one","status":"online","enabled":true}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
		}
	}))
	defer srv.Close()
	config := testutil.WriteConfig(t, srv.URL)

	for _, test := range []struct {
		want map[string]any
		args []string
	}{
		{args: []string{"--environment", "grp-2"}, want: map[string]any{"groupId": "grp-2"}},
		{args: []string{"--environment", ""}, want: map[string]any{"groupId": ""}},
		{args: []string{"--name", "Renamed"}, want: map[string]any{"name": "Renamed"}},
	} {
		out, errOut, err := testutil.Execute(t, append([]string{"--config", config, "agents", "update", "env-1"}, test.args...))
		require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
		require.Equal(t, test.want, updateBody, test.args)
	}
}

func TestEnvironmentsListShowsEnvironmentName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/agents" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"env-1","name":"web-01","apiUrl":"agent://web","status":"online","enabled":true,` +
			`"groupId":"grp-1","group":{"id":"grp-1","name":"Prod","description":""}}],"pagination":{"totalItems":1,"currentPage":1,"itemsPerPage":20}}`))
	}))
	defer srv.Close()

	out, errOut, err := testutil.Execute(t, []string{"--config", testutil.WriteConfig(t, srv.URL), "agents", "list"})
	require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
	require.Contains(t, out, "ENVIRONMENT")
	require.Contains(t, out, "Prod")
	require.NotContains(t, out, "grp-1")
}

func TestEnvironmentsTunnelRestartPrintsSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/agents/env-1/tunnel/restart" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"reconnected":true,"sessionId":"session-new"}}`))
	}))
	defer srv.Close()

	out, errOut, err := testutil.Execute(t, []string{"--config", testutil.WriteConfig(t, srv.URL), "agents", "tunnel", "restart", "env-1"})
	require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
	require.Contains(t, out, "Agent tunnel reconnected")
	require.Contains(t, out, "session-new")
}
