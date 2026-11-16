package fsmeta

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// replaceWithLink makes link, a path that no longer exists, a junction to
// the directory target.
func replaceWithLink(t *testing.T, target, link string) {
	t.Helper()
	makeJunction(t, syspath.ToNative(target), syspath.ToNative(link))
}
