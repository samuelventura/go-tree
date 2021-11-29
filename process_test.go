package tree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessCloseOnReturn(t *testing.T) {
	count := testCount{}
	node := NewRoot("test", testPrinln)
	defer func() {
		assert.Equal(t, count.value, 2)
	}()
	defer node.WaitDisposed()
	count.value = 1
	node.AddAction("action", func() { count.value = 2 })
	node.AddProcess("process", func() {})
}

func TestProcessCloseOnPanic(t *testing.T) {
	count := testCount{}
	node := NewRoot("test", testPrinln)
	defer func() {
		assert.Equal(t, count.value, 2)
	}()
	defer node.WaitDisposed()
	count.value = 1
	node.AddAction("action", func() { count.value = 2 })
	node.AddProcess("process", func() { panic("some error") })
}
