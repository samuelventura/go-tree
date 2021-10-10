package tree

import "container/list"

type Sorted interface {
	Reversed() []string
	Get(string) interface{}
	Set(string, interface{})
	Remove(string) interface{}
}

type named struct {
	name  string
	value interface{}
}

type sorted struct {
	list  *list.List
	index map[string]*list.Element
}

func NewSorted() Sorted {
	dso := &sorted{}
	dso.list = list.New()
	dso.index = make(map[string]*list.Element)
	return dso
}

func (dso *sorted) Reversed() []string {
	names := make([]string, 0, len(dso.index))
	element := dso.list.Back()
	for element != nil {
		item := element.Value.(*named)
		names = append(names, item.name)
		element = element.Prev()
	}
	return names
}

func (dso *sorted) Set(name string, value interface{}) {
	item := &named{name, value}
	dso.index[name] = dso.list.PushBack(item)
}

func (dso *sorted) Get(name string) interface{} {
	element, ok := dso.index[name]
	if ok {
		item := element.Value.(*named)
		return item.value
	}
	return nil
}

func (dso *sorted) Remove(name string) interface{} {
	element, ok := dso.index[name]
	if ok {
		delete(dso.index, name)
		item := dso.list.Remove(element).(*named)
		return item.value
	}
	return nil
}
