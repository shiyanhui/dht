package dht

import (
	"sync"
	"testing"
)

func TestSyncedMap(t *testing.T) {
	cases := []mapItem{
		{"a", 0},
		{"b", 1},
		{"c", 2},
	}

	sm := newSyncedMap()

	set := func() {
		group := sync.WaitGroup{}
		for _, item := range cases {
			group.Add(1)
			go func(item mapItem) {
				sm.Set(item.key, item.val)
				group.Done()
			}(item)
		}
		group.Wait()
	}

	isEmpty := func() {
		if sm.Len() != 0 {
			t.Fail()
		}
	}

	// Set
	set()
	if sm.Len() != len(cases) {
		t.Fail()
	}

Loop:
	// Iter
	for item := range sm.Iter() {
		for _, c := range cases {
			if item.key == c.key && item.val == c.val {
				continue Loop
			}
		}
		t.Fail()
	}

	// Get, Delete, Has
	for _, item := range cases {
		val, ok := sm.Get(item.key)
		if !ok || val != item.val {
			t.Fail()
		}

		sm.Delete(item.key)
		if sm.Has(item.key) {
			t.Fail()
		}
	}
	isEmpty()

	// DeleteMulti
	set()
	sm.DeleteMulti([]interface{}{"a", "b", "c"})
	isEmpty()

	// Clear
	set()
	sm.Clear()
	isEmpty()
}

func TestSyncedList(t *testing.T) {
	sl := newSyncedList()

	insert := func() {
		for i := 0; i < 10; i++ {
			sl.PushBack(i)
		}
	}

	isEmpty := func() {
		if sl.Len() != 0 {
			t.Fail()
		}
	}

	// PushBack
	insert()

	// Len
	if sl.Len() != 10 {
		t.Fail()
	}

	// Iter
	i := 0
	for item := range sl.Iter() {
		if item.Value.(int) != i {
			t.Fail()
		}
		i++
	}

	// Front
	if sl.Front().Value.(int) != 0 {
		t.Fail()
	}

	// Back
	if sl.Back().Value.(int) != 9 {
		t.Fail()
	}

	// Remove
	for i := 0; i < 10; i++ {
		if sl.Remove(sl.Front()).(int) != i {
			t.Fail()
		}
	}
	isEmpty()

	// Clear
	insert()
	sl.Clear()
	isEmpty()
}

func TestKeyedDeque(t *testing.T) {
	cases := []mapItem{
		{"a", 0},
		{"b", 1},
		{"c", 2},
	}

	deque := newKeyedDeque()

	insert := func() {
		for _, item := range cases {
			deque.Push(item.key, item.val)
		}
	}

	isEmpty := func() {
		if deque.Len() != 0 {
			t.Fail()
		}
	}

	// Push
	insert()

	// Len
	if deque.Len() != 3 {
		t.Fail()
	}

	// Iter
	i := 0
	for e := range deque.Iter() {
		if e.Value.(int) != i {
			t.Fail()
		}
		i++
	}

	// HasKey, Get, Delete
	for _, item := range cases {
		if !deque.HasKey(item.key) {
			t.Fail()
		}

		e, ok := deque.Get(item.key)
		if !ok || e.Value.(int) != item.val {
			t.Fail()
		}

		if deque.Delete(item.key) != item.val {
			t.Fail()
		}

		if deque.HasKey(item.key) {
			t.Fail()
		}
	}
	isEmpty()

	// Clear
	insert()
	deque.Clear()
	isEmpty()
}
