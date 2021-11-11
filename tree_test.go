package tree

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTreeRootChannelCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	wait_disposed_and_dump(t, tlog, root, 1000)
	<-go1 //check channel gets closed
}

func TestTreeRootActionCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	go1 := make(chan interface{})
	root.AddAction("go1", func() { close(go1) })
	wait_disposed_and_dump(t, tlog, root, 1000)
	<-go1 //check action gets fired
}

func TestTreeRootCloserCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	go1 := make(chan interface{})
	root.AddCloser("go1", func() error {
		close(go1)
		return nil
	})
	wait_disposed_and_dump(t, tlog, root, 1000)
	<-go1 //check action gets called
}

func TestTreeRootDuplicates(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	root.AddChild("child")
	root.AddProcess("process", func() { <-go1 })
	assert.Panics(t, func() { root.AddChild("child") })
	assert.Equal(t, 1, len(root.State().Children))
	assert.Panics(t, func() { root.AddProcess("process", func() {}) })
	assert.Equal(t, 1, len(root.State().Processes))
	wait_disposed_and_dump(t, tlog, root, 1000)
}

func TestTreeRootClosed(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	root.Close()
	go1 := make(chan interface{})
	root.AddChannel("go1", go1)
	<-go1 //should be closed inmmediately
	go2 := make(chan interface{})
	root.AddCloser("go2", func() error {
		close(go2)
		return nil
	})
	<-go2 //should be called inmmediately
	go3 := make(chan interface{})
	root.AddAction("go3", func() {
		close(go3)
	})
	<-go3 //should be called inmmediately
	assert.Panics(t, func() { root.AddChild("child") })
	assert.Equal(t, 0, len(root.State().Children))
	assert.Panics(t, func() { root.AddProcess("process", func() {}) })
	assert.Equal(t, 0, len(root.State().Processes))
}

func TestTreeChildCleanup(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	assert.Equal(t, "test", root.Name())
	child1 := root.AddChild("child1")
	child2 := root.AddChild("child2")
	child3 := root.AddChild("child3")
	assert.Equal(t, "child1", child1.Name())
	assert.Equal(t, "child2", child2.Name())
	assert.Equal(t, "child3", child3.Name())
	child1.AddProcess("go1", func() { <-child1.Closed() })
	child1.AddProcess("go2", func() { <-child2.Closed() })
	child1.AddProcess("go3", func() { <-child3.Closed() })
	child2.AddProcess("go1", func() { <-child1.Closed() })
	child2.AddProcess("go2", func() { <-child2.Closed() })
	child2.AddProcess("go3", func() { <-child3.Closed() })
	child3.AddProcess("go1", func() { <-child1.Closed() })
	child3.AddProcess("go2", func() { <-child2.Closed() })
	child3.AddProcess("go3", func() { <-child3.Closed() })
	wait_disposed_and_dump(t, tlog, root, 1000)
	assert.Equal(t, 0, len(root.State().Children))
	assert.Equal(t, 0, len(child1.State().Processes))
	assert.Equal(t, 0, len(child2.State().Processes))
	assert.Equal(t, 0, len(child3.State().Processes))
}

func TestTreeRandom(t *testing.T) {
	tlog := &tlog{make(chan string, 1024)}
	root := NewRoot("test", tlog.pln)
	random_populator(root, 5, 10)
	wait_disposed_and_dump(t, tlog, root, 4000)
	assert.Equal(t, 0, len(root.State().Children))
	assert.Equal(t, 0, len(root.State().Processes))
	assert.Equal(t, 0, len(root.State().Actions))
}

func random_populator(node Node, levels_to_go int, max_items int) {
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(max_items)
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
	n = rand.Intn(max_items)
	for i := 0; i < n; i++ {
		node.AddCloser("closer"+fmt.Sprint(i), func() error { return nil })
	}
	n = rand.Intn(max_items)
	for i := 0; i < n; i++ {
		node.AddAction("action"+fmt.Sprint(i), func() {})
	}
	n = rand.Intn(max_items)
	for i := 0; i < n; i++ {
		node.AddProcess("process"+fmt.Sprint(i), func() { <-rch() })
	}
	if levels_to_go <= 0 {
		return
	}
	n = rand.Intn(max_items)
	for i := 0; i < n; i++ {
		child := node.AddChild("child" + fmt.Sprint(i))
		random_populator(child, levels_to_go-1, max_items)
	}
}

func wait_disposed_and_dump(t *testing.T, tlog *tlog, root Node, millis int) {
	defer dump(root, "")
	go tlog.wait_and_print(root.Disposed(), "done")
	go root.Close()
	tlog.wait_to_equal(t, millis, "done\n")
}

func dump(node Node, p string) {
	s := node.State()
	fmt.Println(p, "node", s.Name)
	fmt.Println(p, "actions", len(s.Actions))
	for _, n := range s.Actions {
		fmt.Println(p, "action", n)
	}
	fmt.Println(p, "processes", len(s.Processes))
	for _, n := range s.Processes {
		fmt.Println(p, "process", n)
	}
	fmt.Println(p, "children", len(s.Children))
	for _, n := range s.Children {
		fmt.Println(p, "child", n.Name())
		dump(n, p+"\t")
	}
}
