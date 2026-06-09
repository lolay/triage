// Package updatecheck implements the cached GitHub releases/latest staleness
// banner described in specs/product.md §7.2.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	githubLatestURL = "https://api.github.com/repos/lolay/triage/releases/latest"
	requestTimeout  = 300 * time.Millisecond
	cacheTTL        = 24 * time.Hour
)

// Result holds a pending update notice, if any.
type Result struct {
	Latest  string
	Current string
	Hint    string
}

// Options configures an update check run.
// Field order is optimised to minimise the GC pointer-scan range.
type Options struct {
	HTTPClient     *http.Client     // default: 300ms timeout client
	Now            func() time.Time // default: time.Now
	IsBrewInstall  func() bool      // default: brew list triage
	CurrentVersion string
	CacheDir       string // default: os.UserCacheDir()/triage
	LatestURL      string // default: githubLatestURL (override in tests)
}

// Notice returns an update Result when a newer release exists, or nil when the
// check should be skipped or fails silently.
func Notice(ctx context.Context, opts Options) *Result {
	if !isReleaseBuild(opts.CurrentVersion) {
		return nil
	}

	current, err := semver.NewVersion(normalizeVersion(opts.CurrentVersion))
	if err != nil {
		return nil
	}

	cachePath := cacheFilePath(opts.CacheDir)
	if cached, ok := readCache(cachePath, opts.Now); ok {
		latest, cacheErr := semver.NewVersion(normalizeVersion(cached))
		if cacheErr != nil {
			return nil
		}
		return compare(current, latest, opts)
	}

	latestStr, err := fetchLatest(ctx, opts.HTTPClient, opts.LatestURL)
	if err != nil {
		return nil
	}
	writeCache(cachePath, latestStr, opts.Now)

	latest, err := semver.NewVersion(normalizeVersion(latestStr))
	if err != nil {
		return nil
	}
	return compare(current, latest, opts)
}

func compare(current, latest *semver.Version, opts Options) *Result {
	if !latest.GreaterThan(current) {
		return nil
	}
	hint := "brew install lolay/tap/triage"
	isBrew := brewInstalled
	if opts.IsBrewInstall != nil {
		isBrew = opts.IsBrewInstall
	}
	if isBrew() {
		hint = "brew upgrade triage"
	}
	return &Result{
		Current: current.String(),
		Latest:  latest.String(),
		Hint:    hint,
	}
}

func isReleaseBuild(version string) bool {
	if version == "" || version == "dev" {
		return false
	}
	if strings.Contains(version, "dirty") {
		return false
	}
	if strings.Contains(version, "snapshot") {
		return false
	}
	// git describe dev builds: 0.1.0-5-gabc1234 or semver metadata 0.1.0+abc1234
	if strings.Contains(version, "+") {
		return false
	}
	if strings.Count(version, "-") > 0 {
		return false
	}
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	_, err := semver.NewVersion(v)
	return err == nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}
	return v
}

// field order is optimised to minimise the GC pointer-scan range.
type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func cacheFilePath(dir string) string {
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		dir = filepath.Join(base, "triage")
	}
	return filepath.Join(dir, "update-check.json")
}

func readCache(path string, nowFn func() time.Time) (string, bool) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}
	now := time.Now
	if nowFn != nil {
		now = nowFn
	}
	if now().Sub(entry.CheckedAt) > cacheTTL {
		return "", false
	}
	if entry.Latest == "" {
		return "", false
	}
	return entry.Latest, true
}

func writeCache(path, latest string, nowFn func() time.Time) {
	now := time.Now
	if nowFn != nil {
		now = nowFn
	}
	entry := cacheEntry{Latest: latest, CheckedAt: now()}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o750)
	_ = os.WriteFile(path, data, 0o600)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatest(ctx context.Context, client *http.Client, latestURL string) (string, error) {
	if latestURL == "" {
		latestURL = githubLatestURL
	}
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "triage-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", errors.New("empty tag_name")
	}
	return release.TagName, nil
}

func brewInstalled() bool {
	if _, err := exec.LookPath("brew"); err != nil {
		return false
	}
	cmd := exec.Command("brew", "list", "triage")
	return cmd.Run() == nil
}

// FormatBanner renders the one-line human notice.
func (r *Result) FormatBanner() string {
	return fmt.Sprintf("[!] triage %s available (you have %s). Update: %s",
		r.Latest, r.Current, r.Hint)
}

// ShouldCheck reports whether the update banner may run for this invocation.
func ShouldCheck(noUpdateCheckFlag bool, jsonMode, isTTY, inCI bool) bool {
	if noUpdateCheckFlag || jsonMode || !isTTY || inCI {
		return false
	}
	if os.Getenv("TRIAGE_NO_UPDATE_CHECK") == "1" {
		return false
	}
	return true
}

// InCI reports common CI environment markers.
func InCI() bool {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
