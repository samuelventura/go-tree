package tree

import (
	"fmt"
	"testing"
)

func TestRootChannelCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	check(t, tlog, root)
}

func TestChildCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	child1 := root.Child("child1")
	child2 := root.Child("child2")
	child3 := root.Child("child3")
	child1.Go("go1", func() { <-child1.Closed() })
	child1.Go("go2", func() { <-child2.Closed() })
	child1.Go("go3", func() { <-child3.Closed() })
	child2.Go("go1", func() { <-child1.Closed() })
	child2.Go("go2", func() { <-child2.Closed() })
	child2.Go("go3", func() { <-child3.Closed() })
	child3.Go("go1", func() { <-child1.Closed() })
	child3.Go("go2", func() { <-child2.Closed() })
	child3.Go("go3", func() { <-child3.Closed() })
	check(t, tlog, root)
}

func check(t *testing.T, tlog *tlog, root Node) {
	go tlog.w(root.Done(), "done")
	defer dump(root, "")
	go root.Close()
	tlog.tose(t, 1000, "done\n")
}

func dump(node Node, p string) {
	s := node.State()
	fmt.Println(p, "node", s.Name)
	fmt.Println(p, "actions", len(s.Actions))
	for _, n := range s.Actions {
		fmt.Println(p, "action", n)
	}
	fmt.Println(p, "agents", len(s.Agents))
	for _, n := range s.Agents {
		fmt.Println(p, "agent", n)
	}
	fmt.Println(p, "children", len(s.Children))
	for _, n := range s.Children {
		fmt.Println(p, "child", n.Name())
		dump(n, p+"\t")
	}
}
