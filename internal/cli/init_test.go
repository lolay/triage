package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		platform string
		want     string
	}{
		{platform: "linux", want: "tool: python3"},
		{platform: "macos", want: "tool: python3"},
		{platform: "windows", want: "tool: python"},
	}

	for _, tc := range tests {
		t.Run(tc.platform, func(t *testing.T) {
			t.Parallel()
			got := initTemplate(tc.platform)
			assert.Contains(t, got, tc.want)
		})
	}
}

func TestRunInit(t *testing.T) {
	tests := []struct {
		name       string
		configArg  string
		goos       string
		existing   string
		wantExit   int
		wantStdout string
		wantErr    string
		wantFile   string
		wantMarker string
	}{
		{
			name:       "default",
			goos:       "linux",
			wantExit:   ExitOK,
			wantStdout: "triage: created triage.yaml\n",
			wantFile:   "triage.yaml",
			wantMarker: "tool: python3",
		},
		{
			name:       "custom-config",
			configArg:  "my-triage.yml",
			goos:       "linux",
			wantExit:   ExitOK,
			wantStdout: "triage: created my-triage.yml\n",
			wantFile:   "my-triage.yml",
			wantMarker: "tool: python3",
		},
		{
			name:       "custom-config-with-existing-default",
			configArg:  "my-triage.yml",
			goos:       "linux",
			existing:   "triage.yaml",
			wantExit:   ExitOK,
			wantStdout: "triage: created my-triage.yml\n",
			wantFile:   "my-triage.yml",
			wantMarker: "tool: python3",
		},
		{
			name:       "windows",
			goos:       "windows",
			wantExit:   ExitOK,
			wantStdout: "triage: created triage.yaml\n",
			wantFile:   "triage.yaml",
			wantMarker: "tool: pwsh",
		},
		{
			name:     "already-exists-default",
			goos:     "linux",
			existing: "triage.yml",
			wantExit: ExitUsageError,
			wantErr:  "triage: triage.yml already exists\n",
		},
		{
			name:       "already-exists-custom",
			configArg:  "my-triage.yml",
			goos:       "linux",
			existing:   "my-triage.yml",
			wantExit:   ExitUsageError,
			wantErr:    "triage: my-triage.yml already exists\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			if tc.existing != "" {
				require.NoError(t, os.WriteFile(tc.existing, []byte("default: []\n"), 0o644))
			}

			t.Setenv("TRIAGE_TEST_GOOS", tc.goos)

			var stdout, stderr bytes.Buffer
			exitCode := ExitOK
			err := runInit(tc.configArg, &stdout, &stderr, &exitCode)
			require.NoError(t, err)
			assert.Equal(t, tc.wantExit, exitCode)
			assert.Equal(t, tc.wantStdout, stdout.String())
			assert.Equal(t, tc.wantErr, stderr.String())

			if tc.wantExit == ExitOK {
				data, readErr := os.ReadFile(filepath.Join(dir, tc.wantFile))
				require.NoError(t, readErr)
				assert.Contains(t, string(data), tc.wantMarker)
			}
		})
	}
}

func TestExecuteWithInitDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TRIAGE_TEST_GOOS", "linux")

	exitCode := ExecuteWith([]string{"--init"}, &bytes.Buffer{}, &bytes.Buffer{})
	assert.Equal(t, ExitOK, exitCode)
	_, err := os.Stat("triage.yaml")
	require.NoError(t, err)
}

func TestExecuteWithInitCustomConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TRIAGE_TEST_GOOS", "linux")

	exitCode := ExecuteWith([]string{"--init", "custom.yaml"}, &bytes.Buffer{}, &bytes.Buffer{})
	assert.Equal(t, ExitOK, exitCode)

	data, err := os.ReadFile("custom.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(data), "tool: python3")
}
