package main

type Node struct {
	data string
	next *Node
}

type Queue struct {
	size  int
	front *Node
	rear  *Node
}

func (q *Queue) push(link string) {
	n := &Node{data: link}
	if q.size == 0 {
		q.front = n
		q.rear = n
	} else {
		q.rear.next = n
		q.rear = n
	}
	q.size++
}

// can only pop when not empty, (result, successful)
func (q *Queue) pop() (string, bool) {
	if q.size == 0 {
		return "", false
	}
	res := q.front.data
	q.front = q.front.next
	q.size--
	if q.size == 0 {
		q.rear = nil
	}
	return res, true
}
