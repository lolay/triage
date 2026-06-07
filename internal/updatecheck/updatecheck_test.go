package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotice_skipsDevBuild(t *testing.T) {
	assert.Nil(t, Notice(context.Background(), Options{CurrentVersion: "dev"}))
	assert.Nil(t, Notice(context.Background(), Options{CurrentVersion: "0.1.0+abc1234"}))
	assert.Nil(t, Notice(context.Background(), Options{CurrentVersion: "0.1.0-5-gabc1234"}))
}

func TestNotice_usesCache(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(dir, "update-check.json")
	require.NoError(t, os.WriteFile(cachePath, []byte(`{"latest":"v0.2.0","checked_at":"2026-06-07T11:00:00Z"}`), 0o644))

	got := Notice(context.Background(), Options{
		CurrentVersion: "0.1.0",
		CacheDir:       dir,
		Now:            func() time.Time { return now },
		IsBrewInstall:  func() bool { return true },
	})
	require.NotNil(t, got)
	assert.Equal(t, "0.2.0", got.Latest)
	assert.Equal(t, "0.1.0", got.Current)
	assert.Equal(t, "brew upgrade triage", got.Hint)
}

func TestNotice_fetchesLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0"}`))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	got := Notice(context.Background(), Options{
		CurrentVersion: "0.1.0",
		CacheDir:       dir,
		LatestURL:      srv.URL,
		HTTPClient:     srv.Client(),
		Now:            func() time.Time { return now },
	})
	require.NotNil(t, got)
	assert.Equal(t, "0.3.0", got.Latest)
}

func TestNotice_noUpdateWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(dir, "update-check.json")
	require.NoError(t, os.WriteFile(cachePath, []byte(`{"latest":"v0.1.0","checked_at":"2026-06-07T11:00:00Z"}`), 0o644))

	got := Notice(context.Background(), Options{
		CurrentVersion: "0.1.0",
		CacheDir:       dir,
		Now:            func() time.Time { return now },
	})
	assert.Nil(t, got)
}

func TestShouldCheck(t *testing.T) {
	assert.False(t, ShouldCheck(true, false, true, false))
	assert.False(t, ShouldCheck(false, true, true, false))
	assert.False(t, ShouldCheck(false, false, false, false))
	assert.False(t, ShouldCheck(false, false, true, true))
}

func TestShouldCheck_env(t *testing.T) {
	t.Setenv("TRIAGE_NO_UPDATE_CHECK", "1")
	assert.False(t, ShouldCheck(false, false, true, false))
}
