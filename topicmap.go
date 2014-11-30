package main

import (
	"sync"

	"github.com/tv42/topic"
)

// The pub/sub topic map struct.
type TopicMapEntry struct {
	t     *topic.Topic
	count uint64
}

type TopicMapStruct struct {
	sync.RWMutex
	m map[string]TopicMapEntry
}

func (tms *TopicMapStruct) Register(key string) chan interface{} {
	tms.Lock()
	t, ok := tms.m[key]
	if !ok {
		t = TopicMapEntry{topic.New(), 0}
		tms.m[key] = t
	}
	t.count++
	tms.Unlock()

	ch := make(chan interface{})
	t.t.Register(ch)
	return ch
}

func (tms *TopicMapStruct) Unregister(key string, ch chan<- interface{}) {
	tms.Lock()
	defer tms.Unlock()

	t, ok := tms.m[key]
	if !ok {
		return
	}
	t.t.Unregister(ch)
	t.count--
	if t.count == 0 {
		delete(tms.m, key)
	}
}

var TopicMap = TopicMapStruct{m: make(map[string]TopicMapEntry)}
