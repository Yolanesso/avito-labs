package positions

import (
	"bytes"
	"container/heap"
)

// taskQueue — min-heap: самая старая координата первая, при равенстве — меньший ID.
// Все операции, включая чтение index, выполняются под Pool.mu.
type taskQueue []*task

func (q taskQueue) Len() int { return len(q) }

func (q taskQueue) Less(i, j int) bool { return older(q[i], q[j]) }

func older(a, b *task) bool {
	if !a.trip.PositionAt.Equal(b.trip.PositionAt) {
		return a.trip.PositionAt.Before(b.trip.PositionAt)
	}
	return bytes.Compare(a.trip.ID[:], b.trip.ID[:]) < 0
}

func (q taskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index, q[j].index = i, j
}

func (q *taskQueue) Push(value any) {
	job := value.(*task)
	job.index = len(*q)
	*q = append(*q, job)
}

func (q *taskQueue) Pop() any {
	last := len(*q) - 1
	job := (*q)[last]
	(*q)[last] = nil
	*q = (*q)[:last]
	job.index = -1
	return job
}

// replaceNewest вызывается только при полной очереди. Поиск ограничен её ёмкостью.
// Уже выполняющиеся задачи в heap не входят и никогда не вытесняются.
func (q *taskQueue) replaceNewest(job *task) *task {
	newest := 0
	for i := 1; i < len(*q); i++ {
		if q.Less(newest, i) {
			newest = i
		}
	}
	if !older(job, (*q)[newest]) {
		return nil
	}
	evicted := heap.Remove(q, newest).(*task)
	heap.Push(q, job)
	return evicted
}
