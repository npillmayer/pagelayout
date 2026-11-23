package typography

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Inspect the production graph, not sibling test/optional-adapter imports. A
// module-graph dependency does not itself import a concrete backend package.
func TestDependencyBoundaries(t *testing.T) {
	for _, tc := range []struct{ packages, forbidden []string }{
		{[]string{"."}, []string{"github.com/npillmayer/opentype", "github.com/npillmayer/fontfind", "tytui", "github.com/npillmayer/typography/glyphing/adapter", "github.com/npillmayer/typography/render"}},
		{[]string{"../../content", "../../template", "github.com/npillmayer/css-box-layout/layout"}, []string{"github.com/npillmayer/typography", "github.com/npillmayer/khipu", "github.com/npillmayer/opentype", "github.com/npillmayer/fontfind", "tytui"}},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		args := append([]string{"list", "-deps"}, tc.packages...)
		out, err := exec.CommandContext(ctx, "go", args...).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("inspect dependencies: %v: %s", err, out)
		}
		for _, path := range strings.Fields(string(out)) {
			for _, prefix := range tc.forbidden {
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					t.Fatalf("%v imports %s", tc.packages, path)
				}
			}
		}
	}
}
