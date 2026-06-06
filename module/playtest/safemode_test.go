package playtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldSnapBack(t *testing.T) {
	// Confined AI leaving the sandbox -> snap back.
	assert.True(t, shouldSnapBack(true, "sandbox", []string{"town"}))
	// Confined AI staying in the sandbox -> no snap back.
	assert.False(t, shouldSnapBack(true, "sandbox", []string{"sandbox", "quiet"}))
	// No sandbox tag configured -> never snap back.
	assert.False(t, shouldSnapBack(true, "", []string{"town"}))
	// Non-AI account -> never snap back.
	assert.False(t, shouldSnapBack(false, "sandbox", []string{"town"}))
}
