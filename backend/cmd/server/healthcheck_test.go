package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasHealthcheck(t *testing.T) {
	t.Parallel()
	assert.False(t, hasHealthcheck(nil))
	assert.False(t, hasHealthcheck([]string{"-foo"}))
	assert.True(t, hasHealthcheck([]string{"-healthcheck"}))
}
