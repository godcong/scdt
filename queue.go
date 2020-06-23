package scdt

import "time"

// Queue ...
type Queue struct {
	message  *Message
	callback chan *Message
	timer    *time.Timer
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

// Wait ...
func (q *Queue) Wait() *Message {
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
func DefaultQueue(msg *Message) *Queue {
	return &Queue{
		timer:   time.NewTimer(0),
		message: msg,
	}
}

func CallbackQueue(msg *Message) *Queue {
	return &Queue{
		timer:    time.NewTimer(0),
		callback: make(chan *Message),
		session:  &msg.Session,
		message:  msg,
	}
}
