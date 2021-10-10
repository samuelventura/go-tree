package tree

import (
	"os"
	"sync"
)

type Node interface {
	Name() string
	State() *State
	Child(name string) Node
	Go(name string, action func())
	GetValue(name string) interface{}
	SetValue(name string, value interface{})
	AddAction(name string, action func())
	AddCloser(name string, closer func() error)
	AddChannel(name string, channel chan interface{})
	Closed() <-chan interface{}
	Done() <-chan interface{}
	Close()
	Wait()
}

type State struct {
	Name     string
	Actions  []string
	Agents   []string
	Children []Node
	Closed   bool
	Clean    bool
}

type Log struct {
	Output func(...interface{})
	Fatal  func(...interface{})
}

type node struct {
	log      *Log
	parent   *node
	name     string
	mutex    *sync.Mutex
	actions  Sorted
	children Sorted
	done     chan interface{}
	channel  chan interface{}
	agents   map[string]interface{}
	values   map[string]interface{}
	closed   bool
	clean    bool
}

func NewRoot(log *Log) Node {
	if log == nil {
		output := func(...interface{}) {}
		fatal := func(...interface{}) { os.Exit(1) }
		log = &Log{Output: output, Fatal: fatal}
	}
	dso := &node{}
	dso.log = log
	dso.name = "root"
	dso.mutex = &sync.Mutex{}
	dso.done = make(chan interface{})
	dso.channel = make(chan interface{})
	dso.agents = make(map[string]interface{})
	dso.values = make(map[string]interface{})
	dso.children = NewSorted()
	dso.actions = NewSorted()
	return dso
}

func (dso *node) Name() string {
	return dso.name
}

func (dso *node) Wait() {
	<-dso.done
}

func (dso *node) Done() <-chan interface{} {
	return dso.done
}

func (dso *node) Closed() <-chan interface{} {
	return dso.channel
}

func (dso *node) State() *State {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	s := &State{}
	s.Name = dso.name
	s.Closed = dso.closed
	s.Clean = dso.clean
	s.Actions = dso.actions.Names()
	s.Agents = make([]string, 0, len(dso.agents))
	s.Children = make([]Node, 0, dso.children.Count())
	for n := range dso.agents {
		s.Agents = append(s.Agents, n)
	}
	for _, v := range dso.children.Values() {
		s.Children = append(s.Children, v.(Node))
	}
	return s
}

func (dso *node) Child(name string) Node {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed {
		dso.log.Output("closed ignoring child:", name)
		return nil
	}
	if dso.children.Get(name) != nil {
		dso.log.Fatal("duplicate child:", name)
		return nil
	}
	child := NewRoot(dso.log).(*node)
	child.name = name
	child.parent = dso
	for n, v := range dso.values {
		child.values[n] = v
	}
	dso.children.Set(name, child)
	return child
}

func (dso *node) Go(name string, action func()) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed {
		dso.log.Output("closed ignoring agent:", name)
		return
	}
	if _, ok := dso.agents[name]; ok {
		dso.log.Fatal("duplicate agent:", name)
		return
	}
	dso.agents[name] = action
	go func() {
		defer func() {
			defer dso.cleanup()
			dso.mutex.Lock()
			defer dso.mutex.Unlock()
			delete(dso.agents, name)
		}()
		dso.safe(name, action)
	}()
}

func (dso *node) GetValue(name string) interface{} {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	value, ok := dso.values[name]
	if ok {
		return value
	}
	dso.log.Fatal("value not found:", name)
	return nil
}

func (dso *node) SetValue(name string, value interface{}) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	dso.values[name] = value
}

func (dso *node) AddAction(name string, action func()) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	dso.set(name, action)
}

func (dso *node) AddChannel(name string, channel chan interface{}) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	dso.set(name, func() { close(channel) })
}

func (dso *node) AddCloser(name string, action func() error) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	dso.set(name, func() {
		err := action()
		if err != nil {
			dso.log.Output(name, err)
		}
	})
}

func (dso *node) Close() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed {
		dso.closed = true
		close(dso.channel)
		for _, name := range dso.actions.Names() {
			current := dso.actions.Remove(name)
			dso.safe(name, current.(func()))
		}
		children := dso.children.Values()
		go func() {
			for _, child := range children {
				child.(Node).Close()
			}
			dso.cleanup()
		}()
	}
}

func (dso *node) cleanup() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed {
		return
	}
	if len(dso.agents) > 0 {
		return
	}
	if dso.children.Count() > 0 {
		return
	}
	if dso.clean {
		return
	}
	dso.clean = true
	close(dso.done)
	if dso.parent == nil {
		return
	}
	defer dso.parent.cleanup()
	dso.parent.mutex.Lock()
	defer dso.parent.mutex.Unlock()
	dso.parent.children.Remove(dso.name)
}

func (dso *node) set(name string, action func()) {
	current := dso.actions.Remove(name)
	if current != nil {
		dso.safe(name, current.(func()))
	}
	if dso.closed {
		dso.safe(name, action)
	} else {
		dso.actions.Set(name, action)
	}
}

func (dso *node) safe(name string, action func()) {
	defer func() {
		r := recover()
		if r != nil {
			dso.log.Output("recover", name, r)
		}
	}()
	action()
}
