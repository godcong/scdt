package scdt

import (
	"context"
	"time"
)

var defaultQueueTimeout = 30 * time.Second

// Queue ...
type Queue struct {
	ctx          context.Context
	cancel       context.CancelFunc
	message      *Message
	msgWaiter    chan *Message
	timeout      time.Duration
	session      *Session //link msg.session => session
	sendCallback func(msg *Message)
	recvCallback func(msg *Message)
}

// RecvCallback ...
func (q *Queue) RecvCallback() func(msg *Message) {
	return q.recvCallback
}

// SetRecvCallback ...
func (q *Queue) SetRecvCallback(recvCallback func(msg *Message)) *Queue {
	q.recvCallback = recvCallback
	return q
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
	return q.msgWaiter != nil
}

// trigger ...
func (q *Queue) trigger(message *Message) {
	if q.recvCallback != nil {
		q.recvCallback(message)
	}
	if q.msgWaiter != nil {
		t := time.NewTimer(q.timeout)
		defer t.Stop()
		select {
		case <-t.C:
		case q.msgWaiter <- message:
		}
	}
}

// Wait ...
func (q *Queue) Wait() *Message {
	if q.msgWaiter != nil {
		t := time.NewTimer(q.timeout)
		defer t.Stop()
		select {
		case <-t.C:
		case cb := <-q.msgWaiter:
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
	defer t.Stop()
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
		msgWaiter:    make(chan *Message),
		sendCallback: nil,
		timeout:      defaultQueueTimeout,
		session:      &msg.Session,
		message:      msg,
	}
}
