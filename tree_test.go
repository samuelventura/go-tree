package tree

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootChannelCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	check(t, tlog, root, 1000)
	<-go1
}

func TestRootActionCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	go1 := make(chan interface{})
	root.AddAction("go1", func() {
		close(go1)
	})
	check(t, tlog, root, 1000)
	<-go1
}

func TestRootCloserCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	go1 := make(chan interface{})
	root.AddCloser("go1", func() error {
		close(go1)
		return nil
	})
	check(t, tlog, root, 1000)
	<-go1
}

func TestRootClosed(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	root.Close()
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	<-go1
	go2 := make(chan interface{})
	root.AddCloser("go2", func() error {
		close(go2)
		return nil
	})
	<-go2
	go3 := make(chan interface{})
	root.AddChannel("go3", go3)
	<-go3
	go4 := make(chan interface{})
	root.AddAction("go4", func() {
		close(go4)
	})
	<-go4
	assert.Nil(t, root.Child("child"))
	assert.Equal(t, 0, len(root.State().Children))
	go5 := make(chan interface{})
	root.Go("agent", func() { <-go5 })
	assert.Equal(t, 0, len(root.State().Agents))
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
	check(t, tlog, root, 1000)
}

func TestRandom(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot(&Log{tlog.pln, tlog.ftl})
	random(root, 5, 10)
	check(t, tlog, root, 4000)
}

func random(node Node, vmax int, hmax int) {
	n := rand.Intn(hmax)
	chs := make([]<-chan interface{}, 0, n+1)
	chs = append(chs, node.Closed())
	for i := 0; i < n; i++ {
		ch := make(chan interface{})
		chs = append(chs, ch)
		node.AddChannel("channel"+fmt.Sprint(i), ch)
	}
	rch := func() <-chan interface{} {
		return chs[rand.Intn(len(chs))]
	}
	n = rand.Intn(hmax)
	for i := 0; i < n; i++ {
		node.AddCloser("closer"+fmt.Sprint(i), func() error { return nil })
	}
	n = rand.Intn(hmax)
	for i := 0; i < n; i++ {
		node.AddAction("action"+fmt.Sprint(i), func() {})
	}
	n = rand.Intn(hmax)
	for i := 0; i < n; i++ {
		node.Go("go"+fmt.Sprint(i), func() { <-rch() })
	}
	if vmax <= 0 {
		return
	}
	n = rand.Intn(hmax)
	for i := 0; i < n; i++ {
		child := node.Child("child" + fmt.Sprint(i))
		random(child, vmax-1, hmax)
	}
}

func check(t *testing.T, tlog *tlog, root Node, millis int) {
	go tlog.w(root.Done(), "done")
	defer dump(root, "")
	go root.Close()
	tlog.tose(t, millis, "done\n")
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
