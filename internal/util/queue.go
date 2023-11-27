package util

type Queue[T any] struct {
	items []T
}

func (q *Queue[T]) Enqueue(item T) {
	q.items = append(q.items, item)
}

func (q *Queue[T]) Dequeue() T {
	var i T
	if len(q.items) == 0 {
		return i
	}
	i = q.items[0]
	q.items = q.items[1:]
	return i
}

func (q *Queue[T]) IsEmpty() bool {
	return q == nil || len(q.items) == 0
}

func (q *Queue[T]) ToList() []T {
	if q == nil {
		return make([]T, 0)
	}
	return q.items
}
