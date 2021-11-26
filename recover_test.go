package tree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPrinln(v ...interface{}) {}

type testCount struct {
	value int
}

func TestRecover(t *testing.T) {
	node := NewRoot("test", testPrinln)
	defer node.WaitDisposed()
	defer node.Recover()
	panic("other panic")
}

func TestAddProcessRecover(t *testing.T) {
	node := NewRoot("test", testPrinln)
	defer node.WaitDisposed()
	node.AddProcess("process", func() {
		panic("process panic")
	})
}

func TestIfRecoverAction(t *testing.T) {
	node := NewRoot("test", testPrinln)
	defer node.WaitDisposed()
	defer node.Recover()
	count := &testCount{}
	count.value++
	defer func() {
		assert.Equal(t, 2, count.value)
	}()
	defer node.IfRecoverAction(func() {
		count.value++
		panic("inner panic") //ignored
	})
	panic("outer panic") //recovered before wait
}

func TestIfRecoverCloser(t *testing.T) {
	node := NewRoot("test", testPrinln)
	defer node.WaitDisposed()
	defer node.Recover()
	count := &testCount{}
	count.value++
	defer func() {
		assert.Equal(t, 2, count.value)
	}()
	defer node.IfRecoverCloser(func() error {
		count.value++
		panic("inner panic") //ignored
	})
	panic("outer panic") //recovered before wait
}

func TestIfRecoverChannel(t *testing.T) {
	node := NewRoot("test", testPrinln)
	defer node.WaitDisposed()
	defer node.Recover()
	ch := make(chan interface{})
	defer func() {
		<-ch //expected closed
	}()
	defer node.IfRecoverChannel(ch)
	panic("outer panic") //recovered before wait
}
