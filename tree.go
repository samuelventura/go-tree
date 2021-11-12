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
	AddCloser(name string, action func() error)
	AddChannel(name string, channel chan interface{})
	IfRecover(actionable interface{})
	Disposed() <-chan interface{}
	Closed() <-chan interface{}
	Println(args ...interface{})
	PanicIfError(err error)
	WaitDisposed()
	WaitClosed()
	Recover()
	Close()
}

type State struct {
	Name      string
	Values    map[string]string
	Actions   []string
	Processes []string
	Children  []Node
	Closed    bool
	Disposed  bool
}

type flag struct {
	channel chan interface{}
	flag    bool
}

type node struct {
	log       func(args ...interface{})
	parent    *node
	name      string
	actions   Reversed
	children  Reversed
	mutex     *sync.Mutex
	values    map[string]interface{}
	processes map[string]interface{}
	disposed  flag
	closed    flag
}

func NewRoot(name string, log func(args ...interface{})) Node {
	dso := &node{}
	dso.log = log
	dso.name = name
	dso.mutex = &sync.Mutex{}
	dso.disposed.channel = make(chan interface{})
	dso.closed.channel = make(chan interface{})
	dso.processes = make(map[string]interface{})
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

func (dso *node) Println(args ...interface{}) {
	dso.log(args...)
}

func (dso *node) PanicIfError(err error) {
	if err != nil {
		panic(err)
	}
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
	s.Processes = make([]string, 0, len(dso.processes))
	s.Children = make([]Node, 0, dso.children.Count())
	for n, v := range dso.values {
		s.Values[n] = fmt.Sprint(v)
	}
	for n := range dso.processes {
		s.Processes = append(s.Processes, n)
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
		dso.panic(dso.name, " AddChild node closed: ", name)
	}
	if dso.children.Get(name) != nil {
		dso.panic(dso.name, " AddChild duplicate name: ", name)
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
		dso.panic(dso.name, " AddProcess node closed: ", name)
	}
	if _, ok := dso.processes[name]; ok {
		dso.panic(dso.name, " AddProcess duplicate name: ", name)
	}
	dso.processes[name] = action
	go func() {
		defer dso.dispose()
		defer func() {
			dso.mutex.Lock()
			defer dso.mutex.Unlock()
			delete(dso.processes, name)
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
	dso.panic(dso.name, " value not found: ", name)
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
	dso.set(name, dso.closer(action))
}

func (dso *node) IfRecover(actionable interface{}) {
	r := recover()
	if r != nil {
		ss := stacktrace()
		dso.log("recover:", ss, r)
		switch v := actionable.(type) {
		case func():
			dso.safe(v)
		case func() error:
			dso.safe(dso.closer(v))
		case chan interface{}:
			dso.safe(func() { close(v) })
		}
		panic("repanic")
	}
}

func (dso *node) Recover() {
	defer dso.Close()
	dso.recover()
}

func (dso *node) Close() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed.flag {
		return
	}
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

func (dso *node) dispose() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed.flag {
		return
	}
	if len(dso.processes) > 0 {
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

func (dso *node) closer(action func() error) func() {
	return func() {
		err := action()
		if err != nil {
			dso.log(dso.name, "closer error:", err)
		}
	}
}

func (dso *node) recover() {
	r := recover()
	if r != nil {
		ss := stacktrace()
		dso.log("recover:", ss, r)
	}
}

func (dso *node) panic(args ...interface{}) {
	panic(fmt.Sprint(args...))
}
