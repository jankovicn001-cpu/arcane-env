package environments_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"getarcane.dev/cli/v3/internal/testutil"
)

func newGroupServer(t *testing.T, requests map[string]map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		key := r.Method + " " + r.URL.Path
		switch key {
		case "GET /api/environment-groups":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"grp-1","name":"Prod","description":"Production","environmentCount":3}]}`))
		case "POST /api/environment-groups", "PATCH /api/environment-groups/grp-1":
			var body map[string]any
			assert.NoError(t, json.UnmarshalRead(r.Body, &body))
			requests[key] = body
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"grp-1","name":"Prod","description":"Production","environmentCount":3}}`))
		case "DELETE /api/environment-groups/grp-1":
			requests[key] = map[string]any{}
			_, _ = w.Write([]byte(`{"success":true,"data":{"message":"Environment deleted successfully"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEnvironmentsListCreateUpdateDelete(t *testing.T) {
	requests := map[string]map[string]any{}
	config := testutil.WriteConfig(t, newGroupServer(t, requests).URL)
	run := func(args ...string) string {
		t.Helper()
		out, errOut, err := testutil.Execute(t, append([]string{"--config", config, "environments"}, args...))
		require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
		return out
	}

	listed := run("list")
	for _, want := range []string{"NAME", "AGENTS", "Prod", "Production", "3"} {
		require.Contains(t, listed, want)
	}

	run("create", "Prod", "--description", "Production")
	require.Equal(t, map[string]any{"name": "Prod", "description": "Production"}, requests["POST /api/environment-groups"])

	run("update", "prod", "--name", "Production")
	require.Equal(t, map[string]any{"name": "Production"}, requests["PATCH /api/environment-groups/grp-1"], "update resolves names and sends only passed flags")

	run("delete", "PROD", "--force")
	require.Contains(t, requests, "DELETE /api/environment-groups/grp-1")

	_, _, err := testutil.Execute(t, []string{"--config", config, "environments", "delete", "missing", "--force"})
	require.ErrorContains(t, err, `environment "missing" not found`)
}
