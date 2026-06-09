package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		OS:             "darwin",
		IsBrewInstall:  func() bool { return true },
	})
	require.NotNil(t, got)
	assert.Equal(t, "0.2.0", got.Latest)
	assert.Equal(t, "0.1.0", got.Current)
	assert.Equal(t, "brew upgrade triage", got.Hint)
}

func TestNotice_windowsScoopHint(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(dir, "update-check.json")
	require.NoError(t, os.WriteFile(cachePath, []byte(`{"latest":"v0.2.0","checked_at":"2026-06-07T11:00:00Z"}`), 0o644))

	got := Notice(context.Background(), Options{
		CurrentVersion: "0.1.0",
		CacheDir:       dir,
		Now:            func() time.Time { return now },
		OS:             "windows",
		IsScoopInstall: func() bool { return true },
	})
	require.NotNil(t, got)
	assert.Equal(t, "scoop update triage", got.Hint)
}

func TestNotice_windowsScoopNotInstalledHint(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(dir, "update-check.json")
	require.NoError(t, os.WriteFile(cachePath, []byte(`{"latest":"v0.2.0","checked_at":"2026-06-07T11:00:00Z"}`), 0o644))

	got := Notice(context.Background(), Options{
		CurrentVersion: "0.1.0",
		CacheDir:       dir,
		Now:            func() time.Time { return now },
		OS:             "windows",
		IsScoopInstall: func() bool { return false },
	})
	require.NotNil(t, got)
	assert.Equal(t, "scoop install lolay/triage", got.Hint)
}

func TestNotice_fetchesLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestFormatBanner(t *testing.T) {
	t.Parallel()
	got := (&Result{
		Latest:  "0.4.0",
		Current: "0.3.0",
		Hint:    "brew upgrade triage",
	}).FormatBanner()
	assert.Equal(t, "[!] triage 0.4.0 available (you have 0.3.0). Update: brew upgrade triage", got)
}

func TestInCI(t *testing.T) {
	tests := []struct {
		env  map[string]string
		name string
		want bool
	}{
		{name: "ci set", env: map[string]string{"CI": "1"}, want: true},
		{name: "github actions", env: map[string]string{"GITHUB_ACTIONS": "1"}, want: true},
		{name: "none set", env: map[string]string{}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL"} {
				if v, ok := tc.env[key]; ok {
					t.Setenv(key, v)
				} else {
					t.Setenv(key, "")
				}
			}
			assert.Equal(t, tc.want, InCI())
		})
	}
}

func TestShouldCheck_true(t *testing.T) {
	prev, existed := os.LookupEnv("TRIAGE_NO_UPDATE_CHECK")
	_ = os.Unsetenv("TRIAGE_NO_UPDATE_CHECK")
	if existed {
		t.Cleanup(func() { _ = os.Setenv("TRIAGE_NO_UPDATE_CHECK", prev) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("TRIAGE_NO_UPDATE_CHECK") })
	}
	assert.True(t, ShouldCheck(false, false, true, false))
}

func TestCacheFilePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.Equal(t, filepath.Join(dir, "update-check.json"), cacheFilePath(dir))

	defaultPath := cacheFilePath("")
	assert.True(t, strings.HasSuffix(defaultPath, filepath.Join("triage", "update-check.json")))
}

func TestNormalizeVersion_metadata(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "0.3.0", normalizeVersion("v0.3.0+build123"))
	assert.Equal(t, "0.3.0", normalizeVersion("0.3.0+meta"))
}

func TestFetchLatest_errors(t *testing.T) {
	t.Parallel()

	t.Run("404", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)

		_, err := fetchLatest(context.Background(), srv.Client(), srv.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("empty tag_name", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name":""}`))
		}))
		t.Cleanup(srv.Close)

		_, err := fetchLatest(context.Background(), srv.Client(), srv.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty tag_name")
	})

	t.Run("bad json", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		t.Cleanup(srv.Close)

		_, err := fetchLatest(context.Background(), srv.Client(), srv.URL)
		require.Error(t, err)
	})
}

func TestIsReleaseBuild_snapshotAndMetadata(t *testing.T) {
	t.Parallel()

	assert.False(t, isReleaseBuild("0.1.0-snapshot"))
	assert.False(t, isReleaseBuild("0.1.0+build"))
}
