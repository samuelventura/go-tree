package tree

import (
	"testing"
)

func TestRootChannelAgent(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	root.Go("go1", func() { <-go1 })
	go tlog.w(root.Done(), "done")
	root.Close()
	tlog.tose(t, 100, "done\n")
}

func TestChildCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	child := root.Child("child1")
	child.Go("go1", func() { <-child.Closed() })
	go tlog.w(root.Done(), "done")
	root.Close()
	tlog.tose(t, 100, "done\n")
}
