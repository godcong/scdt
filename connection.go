package scdt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"net"
	"sync"
	"time"

	"go.uber.org/atomic"
)

type connImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	cfg           *Config
	conn          net.Conn
	hbCheck       *time.Timer
	session       *atomic.Uint32
	callbackStore *sync.Map
	msgCB         func(msg *Message)
	sendQueue     chan *Queue
}

func (c *connImpl) Send(message *Message) error {
	panic("implement me")
}

func (c *connImpl) SendMessage(message *Message) error {
	panic("implement me")
}

func NewConn(conn net.Conn, cfs ...ConfigFunc) Connection {
	cfg := defaultConfig()
	for _, cf := range cfs {
		cf(cfg)
	}
	ctx, cancel := context.WithCancel(context.TODO())
	impl := &connImpl{
		ctx:     ctx,
		cancel:  cancel,
		cfg:     cfg,
		conn:    conn,
		session: atomic.NewUint32(1),
	}
	return runConnection(impl)
}

// AcceptNode ...
func Accept(conn net.Conn, cfs ...ConfigFunc) Connection {
	return NewConn(conn, cfs...)
}

// ConnectNode ...
func Connect(conn net.Conn, cfs ...ConfigFunc) Connection {
	return NewConn(conn, cfs...)
}

func (c *connImpl) run() {

}

func (c *connImpl) recv() {

}

func (c *connImpl) send() {
	var err error
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.hbCheck.C:
			err = c.SendMessage(NewSendMessage(MessageHeartBeat, nil))
			if err != nil {
				return
			}
			if c.cfg.Timeout > 0 {
				c.hbCheck.Reset(c.cfg.Timeout)
			}

		case q := <-c.sendQueue:
			c.addCallback(q)
			err = c.SendMessage(q.message)
			if err != nil {
				return
			}
		}
	}
}

func (c *connImpl) newSession() Session {
	s := c.session.Load()
	if s != math.MaxUint32 {
		c.session.Inc()
	} else {
		c.session.Store(1)
	}
	return (Session)(s)
}

func (c *connImpl) addCallback(queue *Queue) {
	if !queue.NeedCallback() {
		return
	}
	s := c.newSession()
	queue.SetSession(s)
	c.callbackStore.Store(s, queue.Trigger)
}

func (c *connImpl) Close() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// ScanExchange ...
func ScanExchange(scanner *bufio.Scanner, packer ReadPacker) error {
	r := bytes.NewReader(scanner.Bytes())
	return packer.Unpack(r)
}

func dataScan(conn net.Conn) *bufio.Scanner {
	scanner := bufio.NewScanner(conn)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if !atEOF && data[0] == 'v' {
			if len(data) > 12 {
				length := uint64(0)
				err := binary.Read(bytes.NewReader(data[4:12]), binary.BigEndian, &length)
				if err != nil {
					return 0, nil, err
				}
				length += 28
				if int(length) <= len(data) {
					return int(length), data[:int(length)], nil
				}
			}
		}
		return
	})
	return scanner
}

func runConnection(impl *connImpl) Connection {
	impl.hbCheck = time.NewTimer(impl.cfg.Timeout)
	go impl.recv()
	go impl.send()
	return impl
}
