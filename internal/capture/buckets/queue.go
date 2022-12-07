package buckets

import (
	"sync"
	"time"
)

type queue struct {
	sync.Mutex
	q []elem
}

type elem struct {
	created time.Time
	bitrate int
}

func (q *queue) push(v elem) {
	q.Lock()
	defer q.Unlock()

	// if the first element is older than 10 seconds, remove it
	if len(q.q) > 0 && time.Since(q.q[0].created) > 10*time.Second {
		q.q = q.q[1:]
	}
	q.q = append(q.q, v)
}

func (q *queue) len() int {
	q.Lock()
	defer q.Unlock()
	return len(q.q)
}

func (q *queue) avg() int {
	q.Lock()
	defer q.Unlock()
	if len(q.q) == 0 {
		return 0
	}
	sum := 0
	for _, v := range q.q {
		sum += v.bitrate
	}
	return sum / len(q.q)
}

func (q *queue) avgLastN(n int) int {
	if n <= 0 {
		return q.avg()
	}
	q.Lock()
	defer q.Unlock()
	if len(q.q) == 0 {
		return 0
	}
	sum := 0
	for _, v := range q.q[len(q.q)-n:] {
		sum += v.bitrate
	}
	return sum / n
}

func (q *queue) min() int {
	q.Lock()
	defer q.Unlock()
	if len(q.q) == 0 {
		return 0
	}
	min := q.q[0].bitrate
	for _, v := range q.q {
		if v.bitrate < min {
			min = v.bitrate
		}
	}
	return min
}

func (q *queue) max() int {
	q.Lock()
	defer q.Unlock()
	if len(q.q) == 0 {
		return 0
	}
	max := 0
	for _, v := range q.q {
		if v.bitrate > max {
			max = v.bitrate
		}
	}
	return max
}
