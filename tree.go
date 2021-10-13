package tree

import (
	"fmt"
	"sync"
)

type Node interface {
	Name() string
	State() *State
	AddChild(name string) Node
	GetValue(name string) interface{}
	SetValue(name string, value interface{})
	AddProcess(name string, action func())
	AddAction(name string, action func())
	AddCloser(name string, closer func() error)
	AddChannel(name string, channel chan interface{})
	Disposed() <-chan interface{}
	Closed() <-chan interface{}
	WaitDisposed()
	WaitClosed()
	Recover()
	Close()
}

type State struct {
	Name     string
	Values   map[string]string
	Actions  []string
	Agents   []string
	Children []Node
	Closed   bool
	Disposed bool
}

type Log struct {
	Warn    func(args ...interface{})
	Recover func(ss string, args ...interface{})
}

type flag struct {
	channel chan interface{}
	flag    bool
}

type node struct {
	log      *Log
	parent   *node
	name     string
	actions  Reversed
	children Reversed
	mutex    *sync.Mutex
	agents   map[string]interface{}
	values   map[string]interface{}
	disposed flag
	closed   flag
}

func NewRoot(name string, log *Log) Node {
	if log == nil {
		nop := func(...interface{}) {}
		rec := func(string, ...interface{}) {}
		log = &Log{Warn: nop, Recover: rec}
	}
	dso := &node{}
	dso.log = log
	dso.name = name
	dso.mutex = &sync.Mutex{}
	dso.disposed.channel = make(chan interface{})
	dso.closed.channel = make(chan interface{})
	dso.agents = make(map[string]interface{})
	dso.values = make(map[string]interface{})
	dso.children = NewReversed()
	dso.actions = NewReversed()
	return dso
}

func (dso *node) Name() string {
	return dso.name
}

func (dso *node) WaitDisposed() {
	<-dso.disposed.channel
}

func (dso *node) WaitClosed() {
	<-dso.closed.channel
}

func (dso *node) Disposed() <-chan interface{} {
	return dso.disposed.channel
}

func (dso *node) Closed() <-chan interface{} {
	return dso.closed.channel
}

func (dso *node) State() *State {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	s := &State{}
	s.Name = dso.name
	s.Closed = dso.closed.flag
	s.Disposed = dso.disposed.flag
	s.Actions = dso.actions.Names()
	s.Values = make(map[string]string)
	s.Agents = make([]string, 0, len(dso.agents))
	s.Children = make([]Node, 0, dso.children.Count())
	for n, v := range dso.values {
		s.Values[n] = fmt.Sprint(v)
	}
	for n := range dso.agents {
		s.Agents = append(s.Agents, n)
	}
	for _, v := range dso.children.Values() {
		s.Children = append(s.Children, v.(Node))
	}
	return s
}

func (dso *node) AddChild(name string) Node {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed.flag {
		dso.log.Warn("node closed")
		return nil
	}
	if dso.children.Get(name) != nil {
		dso.throw("duplicate child")
		return nil
	}
	child := NewRoot(name, dso.log).(*node)
	child.parent = dso
	for n, v := range dso.values {
		child.values[n] = v
	}
	dso.children.Set(name, child)
	return child
}

func (dso *node) AddProcess(name string, action func()) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed.flag {
		dso.log.Warn("node closed")
		return
	}
	if _, ok := dso.agents[name]; ok {
		dso.throw("duplicate agent")
		return
	}
	dso.agents[name] = action
	go func() {
		defer dso.dispose()
		defer func() {
			dso.mutex.Lock()
			defer dso.mutex.Unlock()
			delete(dso.agents, name)
		}()
		defer dso.Recover()
		action()
	}()
}

func (dso *node) GetValue(name string) interface{} {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	value, ok := dso.values[name]
	if ok {
		return value
	}
	dso.throw("value not found")
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
			dso.log.Warn(err)
		}
	})
}

func (dso *node) Recover() {
	defer dso.Close()
	dso.recover()
}

func (dso *node) Close() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed.flag {
		dso.closed.flag = true
		close(dso.closed.channel)
		for _, name := range dso.actions.Names() {
			current := dso.actions.Remove(name)
			dso.safe(current.(func()))
		}
		children := dso.children.Values()
		go func() {
			for _, child := range children {
				child.(Node).Close()
			}
			dso.dispose()
		}()
	}
}

func (dso *node) dispose() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed.flag {
		return
	}
	if len(dso.agents) > 0 {
		return
	}
	if dso.children.Count() > 0 {
		return
	}
	if dso.disposed.flag {
		return
	}
	dso.disposed.flag = true
	close(dso.disposed.channel)
	if dso.parent == nil {
		return
	}
	defer dso.parent.dispose()
	dso.parent.mutex.Lock()
	defer dso.parent.mutex.Unlock()
	dso.parent.children.Remove(dso.name)
}

func (dso *node) set(name string, action func()) {
	current := dso.actions.Remove(name)
	if current != nil {
		dso.safe(current.(func()))
	}
	if dso.closed.flag {
		dso.safe(action)
	} else {
		dso.actions.Set(name, action)
	}
}

func (dso *node) safe(action func()) {
	defer dso.recover()
	action()
}

func (dso *node) recover() {
	r := recover()
	if r != nil {
		ss := stacktrace()
		dso.log.Recover(ss, r)
	}
}

func (dso *node) throw(msg string) {
	panic(msg)
}
