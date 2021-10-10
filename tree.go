package tree

import (
	"log"
	"sync"
)

type Node interface {
	Child(name string) Node
	Go(name string, action func())
	GetValue(name string) interface{}
	SetValue(name string, value interface{})
	AddAction(name string, action func())
	AddCloser(name string, closer func() error)
	AddChannel(name string, channel chan interface{})
	Closed() chan interface{}
	Close()
	Wait()
}

type node struct {
	parent   *node
	name     string
	mutex    *sync.Mutex
	actions  Sorted
	gc       chan interface{}
	done     chan interface{}
	channel  chan interface{}
	agents   map[string]interface{}
	values   map[string]interface{}
	children map[string]Node
	closed   bool
}

func NewRoot() Node {
	dso := &node{}
	dso.mutex = &sync.Mutex{}
	dso.done = make(chan interface{})
	dso.channel = make(chan interface{})
	dso.agents = make(map[string]interface{})
	dso.values = make(map[string]interface{})
	dso.children = make(map[string]Node)
	dso.actions = NewSorted()
	return dso
}

func (dso *node) Wait() {
	<-dso.done
}

func (dso *node) Close() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed {
		dso.closed = true
		dso.gc = make(chan interface{})
		close(dso.channel)
		for _, name := range dso.actions.Reversed() {
			current := dso.actions.Remove(name)
			dso.safe(name, current.(func()))
		}
		for _, child := range dso.children {
			child.Close()
		}
		go dso.gc_loop()
	}
}

func (dso *node) Child(name string) Node {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if _, ok := dso.children[name]; ok {
		log.Fatalln("duplicate child:", name)
		return nil
	}
	child := NewRoot().(*node)
	child.name = name
	child.parent = dso
	dso.copy(child.values)
	dso.children[name] = child
	return child
}

func (dso *node) Closed() chan interface{} {
	return dso.channel
}

func (dso *node) Go(name string, action func()) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed {
		log.Println("closed ignoring agent:", name)
		return
	}
	if _, ok := dso.agents[name]; ok {
		log.Fatalln("duplicate agent:", name)
		return
	}
	dso.agents[name] = action
	go func() {
		defer func() {
			dso.mutex.Lock()
			defer dso.mutex.Unlock()
			delete(dso.agents, name)
			if dso.closed {
				dso.gc_check()
			}
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
	log.Fatalln("value not found:", name)
	return nil
}

func (dso *node) SetValue(name string, value interface{}) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if dso.closed {
		log.Println("closed ignoring value:", name, value)
		return
	}
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
			log.Println(name, err)
		}
	})
}

func (dso *node) copy(values map[string]interface{}) {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	for n, v := range dso.values {
		values[n] = v
	}
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
			log.Println("recover", name, r)
		}
	}()
	action()
}

func (dso *node) gc_check() {
	dso.gc <- true
}

func (dso *node) gc_loop() {
	dso.gc_try()
	for range dso.gc {
		dso.gc_try()
	}
}

func (dso *node) gc_try() {
	dso.mutex.Lock()
	defer dso.mutex.Unlock()
	if !dso.closed {
		return
	}
	if len(dso.agents) > 0 {
		return
	}
	if len(dso.children) > 0 {
		return
	}
	if dso.parent == nil {
		return
	}
	dso.parent.mutex.Lock()
	defer dso.parent.mutex.Unlock()
	delete(dso.parent.children, dso.name)
	if dso.parent.closed {
		dso.parent.gc_check()
	}
	close(dso.done)
	dso.parent = nil
}
