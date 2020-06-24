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

func (c *connImpl) SendMessage(pack WritePacker) error {
	return pack.Pack(c.conn)
}

func (c *connImpl) run() {

}

func (c *connImpl) recv() {
	scan := dataScan(c.conn)
	for scan.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
			var msg Message
			err := ScanExchange(scan, &msg)
			if err != nil {
				panic(err)
			}
			go c.doRecv(&msg)
		}
	}
	if scan.Err() != nil {
		panic(scan.Err())
	}
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

func (c *connImpl) doRecv(msg *Message) {
	switch msg.requestType {
	case RequestTypeRecv:
		c.recvRequest(msg)
	case RequestTypeSend:
		c.recvResponse(msg)
	default:
		c.Close()
		return
	}
}

var recvReqFunc = map[MessageID]func(src *Message, v interface{}) (msg *Message, err error){
	MessageDataTransfer: recvRequest,
	MessageHeartBeat:    recvRequest,
}

func recvRequest(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewRecvMessage(src.MessageID)
	return
}
func recvRequestDataTransfer(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewRecvMessage(src.MessageID)
	return
}

func recvRequestHearBeat(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewRecvMessage(src.MessageID)
	return
}

func (c *connImpl) recvRequest(msg *Message) {
	//f, b := recvReqFunc[msg.MessageID]
	//if !b {
	//	return
	//}
	//ignore error
	newMsg, _ := recvRequest(msg, nil)
	newMsg.Session = msg.Session
	DefaultQueue(newMsg).Send(c.sendQueue)
}

func (c *connImpl) recvResponse(msg *Message) {
	if msg.MessageID == MessageHeartBeat {
		c.hbCheck.Reset(c.cfg.Timeout)
		return
	}
	trigger, b := c.GetCallback(msg.Session)
	if b {
		trigger(msg)
	}
}

func (c *connImpl) GetCallback(session Session) (f func(message *Message), b bool) {
	if session == 0 {
		return
	}
	load, ok := c.callbackStore.Load(session)
	if ok {
		f, b = load.(func(message *Message))
		c.callbackStore.Delete(session)
	}
	return
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