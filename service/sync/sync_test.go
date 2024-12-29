package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_sync(t *testing.T) {
	asserts := assert.New(t)
	asserts.NotPanics(func() {
		Sync()
	})
}
