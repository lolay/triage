package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lolay/triage/internal/engine"
)

// TestJSON_KeyOrder locks the key order of --json result objects. encoding/json
// emits struct keys in declaration order, so a field reorder (e.g. govet's
// fieldalignment) would silently change the output shape (spec §7.3). This test
// fails if that happens. JSONResult carries a //nolint:govet for the same reason.
func TestJSON_KeyOrder(t *testing.T) {
	var buf bytes.Buffer
	results := []engine.Result{
		{
			Label:    "fake-tool",
			Kind:     engine.KindLeaf,
			Severity: engine.SeverityError,
			Pass:     false,
			Message:  "fake-tool not found — install it",
			Depth:    1,
		},
	}
	require.NoError(t, JSON(&buf, results, "default", ""), "JSON")
	got := buf.String()

	// Among the always-present + populated keys, this is the documented order.
	want := []string{"name", "kind", "depth", "severity", "status", "detail"}
	lastIdx := -1
	for _, key := range want {
		idx := strings.Index(got, `"`+key+`"`)
		require.GreaterOrEqualf(t, idx, 0, "key %q missing from JSON output:\n%s", key, got)
		assert.GreaterOrEqualf(t, idx, lastIdx, "key %q is out of declaration order in JSON output:\n%s", key, got)
		lastIdx = idx
	}
}
