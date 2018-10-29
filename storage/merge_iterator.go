package storage

import (
	"bytes"
	"container/heap"
)

type iterHeap []Iterator

func (h iterHeap) Len() int {
	return len(h)
}

func (h iterHeap) Less(i, j int) bool {
	return bytes.Compare(h[i].Key(), h[j].Key()) == -1
}

func (h iterHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *iterHeap) Push(x interface{}) {
	*h = append(*h, x.(Iterator))
}

func (h *iterHeap) Pop() interface{} {
	dref := *h
	x := dref[len(dref)-1]
	*h = dref[:len(dref)-1]
	return x
}

type mergeIterator struct {
	key   []byte
	value []byte

	err error

	heap  iterHeap
	iters []Iterator
}

// MergeIterator returns a new Iterator that iterates over the given
// subiterators. Iteration happens in lexicographical order given that the
// subiterators also return values in lexicographical order.
func MergeIterator(iters []Iterator) Iterator {
	miter := &mergeIterator{
		iters: iters,
		heap:  make([]Iterator, 0, len(iters)),
	}

	miter.buildHeap(func(i Iterator) bool { return i.Next() })

	return miter
}

func (m *mergeIterator) buildHeap(hasValue func(i Iterator) bool) {
	m.heap = m.heap[:0]

	for _, iter := range m.iters {
		if !hasValue(iter) {
			if m.err = iter.Error(); m.err != nil {
				return
			}

			continue
		}

		heap.Push(&m.heap, iter)
	}
}

// Key returns the current key. Caller should not keep references to the
// buffer or modify its contents.
func (m *mergeIterator) Key() []byte {
	return m.key
}

// Value returns the current value. Caller should not keep references to the
// buffer or modify its contents.
func (m *mergeIterator) Value() []byte {
	return m.value
}

// Seek moves the iterator to a position that is greater or equal to the given
// key. It returns whether such key exists.
func (m *mergeIterator) Seek(key []byte) bool {
	if m.err != nil {
		return false
	}

	m.buildHeap(func(i Iterator) bool { return i.Seek(key) && i.Next() })

	return m.err == nil && len(m.heap) > 0
}

// Seek moves the iterator to the begining of a key-value pair sequence that
// is greater or equal to the given key. It returns whether at least one of
// such key-value pairs exist. Next must be called after seeking to access
// the first pair.
func (m *mergeIterator) Next() bool {
	if m.err != nil || len(m.heap) == 0 {
		return false
	}

	iter := heap.Pop(&m.heap).(Iterator)

	// cache the values as the underlying iterator might reuse its buffers on
	// call to Next
	m.key = append(m.key[:0], iter.Key()...)
	m.value = append(m.value[:0], iter.Value()...)

	if iter.Next() {
		heap.Push(&m.heap, iter)
	} else if m.err = iter.Error(); m.err != nil {
		return false
	}

	return true
}

// Error returns the error encountered during iteration if any.
func (m *mergeIterator) Error() error {
	return m.err
}

// Release frees up the resources used by the iterator. This will also release
// the subiterators.
func (m *mergeIterator) Release() {
	for i := range m.iters {
		m.iters[i].Release()
	}

	m.iters = nil
	m.heap = nil
	m.key = nil
	m.value = nil
}
