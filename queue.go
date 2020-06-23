package scdt

import "time"

// Queue ...
type Queue struct {
	message  *Message
	callback chan *Message
	timer    *time.Timer
	timeout  time.Duration
}

func (q *Queue) Callback() chan *Message {
	return q.callback
}

func (q *Queue) SetCallback(callback chan *Message) {
	q.callback = callback
}

func (q *Queue) TriggerCallback(message *Message) {
	if q.callback != nil {
		if q.timeout != 0 {
			q.timer.Reset(q.timeout)
		}
		select {
		case <-q.timer.C:
		case q.callback <- message:
			q.timer.Reset(0)
		}
	}
}

// WaitCallback ...
func (q *Queue) WaitCallback() *Message {
	if q.callback != nil {
		if q.timeout != 0 {
			q.timer.Reset(q.timeout)
		}
		select {
		case <-q.timer.C:
		case cb := <-q.callback:
			q.timer.Reset(0)
			return cb
		}
	}
	return nil
}

func (q *Queue) Timeout() time.Duration {
	return q.timeout
}

func (q *Queue) SetTimeout(timeout time.Duration) {
	q.timeout = timeout
}

func (q *Queue) Send(out chan<- *Queue) bool {
	if out == nil {
		return false
	}
	if q.timeout != 0 {
		q.timer.Reset(q.timeout)
	}
	select {
	case <-q.timer.C:
		return false
	case out <- q:
		q.timer.Reset(0)
		return true
	}
}

// NewQueue ...
func NewQueue(msg *Message, enableCallback bool) *Queue {
	return &Queue{
		timer:   time.NewTimer(0),
		message: msg,
	}
}
