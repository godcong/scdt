package scdt

import "time"

var defaultQueueTimeout = 30 * time.Second

// Queue ...
type Queue struct {
	message      *Message
	callback     chan *Message
	timeout      time.Duration
	session      *Session //link msg.session => session
	sendCallback func(msg *Message)
}

// Session ...
func (q *Queue) Session() Session {
	return q.message.Session
}

// setSession ...
func (q *Queue) setSession(session Session) {
	if session == 0 || q.session == nil {
		return
	}
	*q.session = session
}

// NeedCallback ...
func (q *Queue) NeedCallback() bool {
	return q.callback != nil
}

// trigger ...
func (q *Queue) trigger(message *Message) {
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

// SetTimeout set timeout with time duration
func (q *Queue) SetTimeout(timeout time.Duration) {
	q.timeout = timeout
}

// send ...
func (q *Queue) send(out chan<- *Queue) bool {
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

// SetSendCallback ...
func (q *Queue) SetSendCallback(f func(message *Message)) *Queue {
	q.sendCallback = f
	return q
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
		callback:     make(chan *Message),
		sendCallback: nil,
		timeout:      defaultQueueTimeout,
		session:      &msg.Session,
		message:      msg,
	}
}
