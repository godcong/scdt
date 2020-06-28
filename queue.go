package scdt

import "time"

var defaultQueueTimeout = 30 * time.Second

// Queue ...
type Queue struct {
	message  *Message
	callback chan *Message
	timeout  time.Duration
	session  *Session //link msg.session => session
}

// Session ...
func (q *Queue) Session() Session {
	return q.message.Session
}

// SetSession ...
func (q *Queue) SetSession(session Session) {
	if session == 0 || q.session == nil {
		return
	}
	*q.session = session
}

// NeedCallback ...
func (q *Queue) NeedCallback() bool {
	return q.callback != nil
}

// Trigger ...
func (q *Queue) Trigger(message *Message) {
	if q.callback != nil {
		t := time.NewTimer(q.timeout)
		defer t.Reset(0)
		select {
		case <-t.C:
		case q.callback <- message:
		}
	}
}

// Wait ...
func (q *Queue) Wait() *Message {
	if q.callback != nil {
		t := time.NewTimer(q.timeout)
		defer t.Reset(0)
		select {
		case <-t.C:
		case cb := <-q.callback:
			return cb
		}
	}
	return nil
}

// Timeout ...
func (q *Queue) Timeout() time.Duration {
	return q.timeout
}

// SetTimeout ...
func (q *Queue) SetTimeout(timeout time.Duration) {
	q.timeout = timeout
}

// Send ...
func (q *Queue) Send(out chan<- *Queue) bool {
	if out == nil {
		return false
	}

	t := time.NewTimer(q.timeout)
	defer t.Reset(0)
	select {
	case <-t.C:
		return false
	case out <- q:
		return true
	}
}

// DefaultQueue ...
func DefaultQueue(msg *Message) *Queue {
	return &Queue{
		timeout: defaultQueueTimeout,
		message: msg,
	}
}

// CallbackQueue ...
func CallbackQueue(msg *Message) *Queue {
	return &Queue{
		callback: make(chan *Message),
		timeout:  defaultQueueTimeout,
		session:  &msg.Session,
		message:  msg,
	}
}
