package usergroups_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"getarcane.dev/cli/v3/internal/testutil"
)

const testGroup = `{"id":"ug-1","name":"Ops","description":"On call","memberCount":4,"links":[{"kind":"oidc","value":"ops"}]}`

func newUserGroupServer(t *testing.T, requests map[string]map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		key := r.Method + " " + r.URL.Path
		switch key {
		case "GET /api/user-groups":
			_, _ = w.Write([]byte(`{"success":true,"data":[` + testGroup + `]}`))
		case "POST /api/user-groups":
			var body map[string]any
			assert.NoError(t, json.UnmarshalRead(r.Body, &body))
			requests[key] = body
			_, _ = w.Write([]byte(`{"success":true,"data":` + testGroup + `}`))
		case "DELETE /api/user-groups/ug-1":
			requests[key] = map[string]any{}
			_, _ = w.Write([]byte(`{"success":true,"data":{"message":"User group deleted successfully"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUserGroupsListCreateDelete(t *testing.T) {
	requests := map[string]map[string]any{}
	config := testutil.WriteConfig(t, newUserGroupServer(t, requests).URL)
	run := func(args ...string) string {
		t.Helper()
		out, errOut, err := testutil.Execute(t, append([]string{"--config", config, "admin", "user-groups"}, args...))
		require.NoError(t, err, "stdout=%s\nstderr=%s", out, errOut)
		return out
	}

	listed := run("list")
	for _, want := range []string{"NAME", "MEMBERS", "LINKS", "Ops", "On call", "4", "oidc:ops"} {
		require.Contains(t, listed, want)
	}

	run("create", "Ops", "--description", "On call", "--oidc", "ops", "--ldap", "cn=ops")
	require.Equal(t, map[string]any{
		"name":        "Ops",
		"description": "On call",
		"links":       []any{map[string]any{"kind": "oidc", "value": "ops"}, map[string]any{"kind": "ldap", "value": "cn=ops"}},
	}, requests["POST /api/user-groups"])

	run("delete", "ops", "--force")
	require.Contains(t, requests, "DELETE /api/user-groups/ug-1", "delete resolves names case-insensitively")

	_, _, err := testutil.Execute(t, []string{"--config", config, "admin", "user-groups", "delete", "missing", "--force"})
	require.ErrorContains(t, err, `user group "missing" not found`)
}
