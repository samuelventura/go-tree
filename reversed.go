package tree

import "container/list"

type Reversed interface {
	Count() int
	Names() []string
	Values() []interface{}
	Get(string) interface{}
	Set(string, interface{})
	Remove(string) interface{}
}

type named struct {
	name  string
	value interface{}
}

type reversed struct {
	list  *list.List
	index map[string]*list.Element
}

func NewReversed() Reversed {
	dso := &reversed{}
	dso.list = list.New()
	dso.index = make(map[string]*list.Element)
	return dso
}

func (dso *reversed) Count() int {
	return len(dso.index)
}

func (dso *reversed) Values() []interface{} {
	values := make([]interface{}, 0, len(dso.index))
	element := dso.list.Front()
	for element != nil {
		item := element.Value.(*named)
		values = append(values, item.value)
		element = element.Next()
	}
	return values
}

func (dso *reversed) Names() []string {
	names := make([]string, 0, len(dso.index))
	element := dso.list.Front()
	for element != nil {
		item := element.Value.(*named)
		names = append(names, item.name)
		element = element.Next()
	}
	return names
}

func (dso *reversed) Set(name string, value interface{}) {
	if _, ok := dso.index[name]; ok {
		panic("duplicated")
	}
	item := &named{name, value}
	dso.index[name] = dso.list.PushFront(item)
}

func (dso *reversed) Get(name string) interface{} {
	element, ok := dso.index[name]
	if ok {
		item := element.Value.(*named)
		return item.value
	}
	return nil
}

func (dso *reversed) Remove(name string) interface{} {
	element, ok := dso.index[name]
	if ok {
		delete(dso.index, name)
		item := dso.list.Remove(element).(*named)
		return item.value
	}
	return nil
}
