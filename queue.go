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

func (q *Queue) Session() Session {
	return q.message.Session
}

func (q *Queue) SetSession(session Session) {
	if session == 0 || q.session == nil {
		return
	}
	*q.session = session
}

func (q *Queue) NeedCallback() bool {
	return q.callback != nil
}

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

	t := time.NewTimer(q.timeout)
	defer t.Reset(0)
	select {
	case <-t.C:
		return false
	case out <- q:
		return true
	}
}

// NewQueue ...
func DefaultQueue(msg *Message) *Queue {
	return &Queue{
		timeout: defaultQueueTimeout,
		message: msg,
	}
}

func CallbackQueue(msg *Message) *Queue {
	return &Queue{
		callback: make(chan *Message),
		timeout:  defaultQueueTimeout,
		session:  &msg.Session,
		message:  msg,
	}
}
