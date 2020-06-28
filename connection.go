package scdt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"sync"
	"time"

	"go.uber.org/atomic"
)

type connImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	fn            MessageCallbackFunc
	cfg           *Config
	localID       string
	remoteID      *atomic.String
	conn          net.Conn
	hbCheck       *time.Timer
	session       *atomic.Uint32
	callbackStore *sync.Map
	recvStore     *sync.Map
	sendQueue     chan *Queue
	closed        *atomic.Bool
}

var defaultConnSendTimeout = 30 * time.Second

// IsClosed ...
func (c *connImpl) IsClosed() bool {
	if c.closed != nil {
		return c.closed.Load()
	}
	return true
}

// LocalID ...
func (c *connImpl) LocalID() string {
	return c.localID
}

// RemoteID ...
func (c *connImpl) RemoteID() (id string, err error) {
	id = c.remoteID.Load()
	if id == "" {
		msg := NewRecvMessage(MessageConnectID)
		msg.SetDataString("hello world")
		queue := CallbackQueue(msg)
		if queue.Send(c.sendQueue) {
			msg := queue.Wait()
			if msg != nil && msg.DataLength != 0 {
				id = string(msg.Data)
				c.remoteID.Store(id)
				return
			}
			return "", errors.New("get response message failed")
		}
		return "", errors.New("send id request failed")
	}
	return id, nil
}

// MessageCallback ...
func (c *connImpl) MessageCallback(fn MessageCallbackFunc) {
	c.fn = fn
}

// NewConn ...
func NewConn(conn net.Conn, cfs ...ConfigFunc) Connection {
	cfg := defaultConfig()
	for _, cf := range cfs {
		cf(cfg)
	}

	ider := UUID
	if cfg.CustomIDer != nil {
		ider = cfg.CustomIDer
	}

	ctx, cancel := context.WithCancel(context.TODO())
	impl := &connImpl{
		ctx:           ctx,
		cancel:        cancel,
		localID:       ider(),
		cfg:           cfg,
		conn:          conn,
		hbCheck:       time.NewTimer(cfg.Timeout),
		session:       atomic.NewUint32(1),
		callbackStore: new(sync.Map),
		recvStore:     new(sync.Map),
		remoteID:      atomic.NewString(""),
		sendQueue:     make(chan *Queue),
		closed:        atomic.NewBool(false),
	}
	return runConnection(impl)
}

// Accept ...
func Accept(conn net.Conn, cfs ...ConfigFunc) Connection {
	return NewConn(conn, cfs...)
}

// Connect ...
func Connect(conn net.Conn, cfs ...ConfigFunc) Connection {
	tmp := []ConfigFunc{func(c *Config) {
		c.Timeout = 30 * time.Second
	}}
	tmp = append(tmp, cfs...)
	return NewConn(conn, tmp...)
}

func (c *connImpl) sendMessage(pack WritePacker) error {
	return pack.Pack(c.conn)
}

func (c *connImpl) recv() {
	defer func() {
		if recover() != nil {
			c.Close()
		}
	}()
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
			log.Debugw("recv", "msg", msg)
			go c.doRecv(&msg)
		}
	}
	if scan.Err() != nil {
		panic(scan.Err())
	}
}

func (c *connImpl) send() {
	defer func() {
		if recover() != nil {
			c.Close()
		}
	}()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.hbCheck.C:
			if c.cfg.Timeout == 0 {
				continue
			}
			err := c.sendMessage(NewRecvMessage(MessageHeartBeat))
			if err != nil {
				panic(err)
			}
			c.hbCheck.Reset(c.cfg.Timeout)
		case q := <-c.sendQueue:
			c.addCallback(q)
			log.Debugw("send", "msg", q.message)
			err := c.sendMessage(q.message)
			if err != nil {
				panic(err)
			}
			if q.sendCallback != nil {
				q.sendCallback(q.message)
			}

		default:
			if c.IsClosed() {
				return
			}
			time.Sleep(30 * time.Millisecond)
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
	log.Infow("add callback", "session", s)
	queue.SetSession(s)
	c.callbackStore.Store(s, queue.Trigger)
}

// Send ...
func (c *connImpl) SendWithCallback(id CustomID, data []byte, cb func(message *Message)) bool {
	return CallbackQueue(NewCustomMessage(id, data)).SetSendCallback(cb).Send(c.sendQueue)
}

// Send ...
func (c *connImpl) Send(id CustomID, data []byte) bool {
	return c.SendWithCallback(id, data, nil)
}

// Close ...
func (c *connImpl) Close() {
	log.Debugw("close")
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.hbCheck.Reset(0)

	if c.closed != nil {
		c.closed.Store(true)
	}
}

func (c *connImpl) doRecv(msg *Message) {
	switch msg.requestType {
	case RequestTypeRecv:
		c.recvRequest(msg)
	case RequestTypeSend:
		c.recvResponse(msg)
	default:
		panic("unsupported request type")
		return
	}
}

var recvReqFunc = map[MessageID]func(src *Message, v interface{}) (msg *Message, err error){
	MessageDataTransfer: recvRequestDataTransfer,
	MessageHeartBeat:    recvRequestHearBeat,
	MessageConnectID:    recvRequestID,
	MessageUserCustom:   recvRequest,
}

func recvRequest(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewSendMessage(src.MessageID, nil)
	return
}
func recvRequestDataTransfer(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewSendMessage(src.MessageID, nil)
	return
}

func recvRequestHearBeat(src *Message, v interface{}) (msg *Message, err error) {
	msg = NewSendMessage(src.MessageID, nil)
	return
}
func recvRequestID(src *Message, v interface{}) (msg *Message, err error) {
	id := v.(string)
	msg = NewSendMessage(src.MessageID, toBytes(id))
	msg.Session = src.Session
	log.Debugw("local", "id", id, "src", src, "target", msg)
	return
}

func (c *connImpl) recvRequest(msg *Message) {
	f, b := recvReqFunc[msg.MessageID]
	if !b {
		return
	}

	//ignore error
	newMsg, _ := f(msg, c.localID)
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

// GetCallback ...
func (c *connImpl) GetCallback(session Session) (f func(message *Message), b bool) {
	if session == 0 {
		return
	}
	log.Infow("load callback", "session", session)
	load, ok := c.callbackStore.Load(session)
	if ok {
		f, b = load.(func(message *Message))
		log.Infow("callback", "has", b)
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
			if len(data) > 16 {
				length := uint64(0)
				err := binary.Read(bytes.NewReader(data[8:16]), binary.BigEndian, &length)
				if err != nil {
					return 0, nil, err
				}
				length += 20
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

func toBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return []byte(s)
}
