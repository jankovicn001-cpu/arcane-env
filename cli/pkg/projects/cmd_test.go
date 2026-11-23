package projects_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/cli/v3/internal/testutil"
)

func TestProjectsListAllIncludesArchived(t *testing.T) {
	var (
		mu       sync.Mutex
		gotQuery string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotQuery = r.URL.RawQuery
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [],
			"pagination": {"totalPages":1,"totalItems":0,"currentPage":1,"itemsPerPage":0}
		}`))
	}))
	defer srv.Close()

	configPath := testutil.WriteConfig(t, srv.URL)
	_, errOut, err := testutil.Execute(
		t,
		[]string{"--config", configPath, "--agent", "0", "projects", "list", "--json", "--all"},
	)

	require.NoError(t, err,
		"execute: %v (%s)", err, errOut)

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, "archived=all&limit=-1", gotQuery,
		"query = %q, want %q", gotQuery, "archived=all&limit=-1")
}
