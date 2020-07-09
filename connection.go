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
	ctx                    context.Context
	cancel                 context.CancelFunc
	fn                     MessageCallbackFunc
	cfg                    *Config
	localID                string
	recvCallback           RecvCallbackFunc
	recvCustomDataCallback RecvCallbackFunc
	remoteID               *atomic.String
	conn                   net.Conn
	hbCheck                *time.Timer
	session                *atomic.Uint32
	callbackStore          *sync.Map
	recvStore              *sync.Map
	sendQueue              chan *Queue
	closed                 *atomic.Bool
}

// RecvCallbackFunc ...
type RecvCallbackFunc func(message *Message) ([]byte, bool)

var defaultConnSendTimeout = 30 * time.Second
var recvReqFunc = map[MessageID]func(src *Message, v interface{}) (msg *Message, err error){
	MessageDataTransfer: recvRequestDataTransfer,
	MessageHeartBeat:    recvRequestHearBeat,
	MessageConnectID:    recvRequestID,
	MessageUserCustom:   recvCustomRequest,
}

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
		msg := newRecvMessage(MessageConnectID)
		msg.SetDataString("hello world")
		queue := CallbackQueue(msg)
		if queue.send(c.sendQueue) {
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
			err := c.sendMessage(newRecvMessage(MessageHeartBeat))
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
	queue.setSession(s)
	c.callbackStore.Store(s, queue.trigger)
}

// SendOnWait ...
func (c *connImpl) SendCustomDataOnWait(id CustomID, data []byte) (msg *Message, b bool) {
	queue := CallbackQueue(newCustomRecvMessage(id, data))
	if b = queue.send(c.sendQueue); b {
		msg = queue.Wait()
	}
	return
}

// send ...
func (c *connImpl) SendCustomData(id CustomID, data []byte) (*Queue, bool) {
	return c.SendCustomDataWithCallback(id, data, nil)
}

// send ...
func (c *connImpl) SendCustomDataWithCallback(id CustomID, data []byte, cb func(message *Message)) (*Queue, bool) {
	queue := CallbackQueue(newCustomRecvMessage(id, data)).SetSendCallback(cb)
	b := queue.send(c.sendQueue)
	return queue, b
}

// send ...
func (c *connImpl) Send(data []byte) (*Queue, bool) {
	return c.SendWithCallback(data, nil)
}

// SendOnWait ...
func (c *connImpl) SendOnWait(data []byte) (msg *Message, b bool) {
	queue := CallbackQueue(newRecvMessage(MessageDataTransfer).SetData(data))
	if b = queue.send(c.sendQueue); b {
		msg = queue.Wait()
	}
	return
}

// send ...
func (c *connImpl) SendWithCallback(data []byte, cb func(message *Message)) (*Queue, bool) {
	queue := CallbackQueue(newRecvMessage(MessageDataTransfer).SetData(data)).SetSendCallback(cb)
	b := queue.send(c.sendQueue)
	return queue, b
}

// send ...
func (c *connImpl) Recv(fn RecvCallbackFunc) {
	c.recvCallback = fn
}

// send ...
func (c *connImpl) RecvCustomData(fn RecvCallbackFunc) {
	c.recvCustomDataCallback = fn
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
	case RequestTypeFailed:
		c.recvFailed(msg)
	default:
		panic("unsupported request type")
		return
	}
}

func recvRequestDataTransfer(src *Message, v interface{}) (msg *Message, err error) {
	if v == nil {
		return newSendMessage(src.MessageID, nil), nil
	}
	fn, b := v.(RecvCallbackFunc)
	if !b {
		return newSendMessage(src.MessageID, nil), nil
	}
	srcCopy := *src
	copy(srcCopy.Data, src.Data)
	data, b := fn(&srcCopy)
	if !b {
		return nil, errors.New("do not need response")
	}
	msg = newSendMessage(src.MessageID, data)
	return
}
func recvRequestFailed(src *Message, v interface{}) (msg *Message, err error) {
	msg = newFailedSendMessage(toBytes(v.(string)))
	return msg, nil
}
func recvRequestHearBeat(src *Message, v interface{}) (msg *Message, err error) {
	msg = newSendMessage(src.MessageID, nil)
	return
}
func recvRequestID(src *Message, v interface{}) (msg *Message, err error) {
	id := v.(string)
	msg = newSendMessage(src.MessageID, toBytes(id))
	log.Debugw("local", "id", id, "src", src, "target", msg)
	return
}
func recvCustomRequest(src *Message, v interface{}) (msg *Message, err error) {
	if v == nil {
		return newCustomSendMessage(src.CustomID, nil), nil
	}
	fn, b := v.(RecvCallbackFunc)
	if !b || fn == nil {
		return newCustomSendMessage(src.CustomID, nil), nil
	}
	//prevent data from being destroyed

	srcCopy := *src
	copy(srcCopy.Data, src.Data)
	log.Debugw("process custom data", "data", srcCopy)
	data, b := fn(&srcCopy)
	if !b {
		return nil, errors.New("do not need response")
	}
	return newCustomSendMessage(src.CustomID, data), nil
}

func (c *connImpl) getMessageArgs(id MessageID) interface{} {
	switch id {
	case MessageConnectID:
		return c.localID
	case MessageDataTransfer:
		return c.recvCallback
	case MessageUserCustom:
		return c.recvCustomDataCallback
	}
	return nil
}

func (c *connImpl) recvRequest(msg *Message) {
	f, b := recvReqFunc[msg.MessageID]
	var newMsg *Message
	var err error
	if b {
		v := c.getMessageArgs(msg.MessageID)
		newMsg, err = f(msg, v)
	} else {
		return
	}
	log.Debugw("received", "msg", newMsg, "err", err)
	if err != nil {
		newMsg, err = recvRequestFailed(msg, err.Error())
		if err != nil {
			return
		}
	}
	newMsg.MessageID = msg.MessageID
	newMsg.Session = msg.Session
	newMsg.CustomID = msg.CustomID
	DefaultQueue(newMsg).send(c.sendQueue)
}

func (c *connImpl) recvResponse(msg *Message) {
	if msg.MessageID == MessageHeartBeat {
		c.hbCheck.Reset(c.cfg.Timeout)
		return
	}
	trigger, b := c.getCallback(msg.Session)
	if b {
		trigger(msg)
	}
}

// getCallback ...
func (c *connImpl) getCallback(session Session) (f func(message *Message), b bool) {
	if session == 0 {
		return
	}
	log.Infow("load callback", "session", session)
	load, ok := c.callbackStore.Load(session)
	if ok {
		f, b = load.(func(message *Message))
		log.Debugw("callback", "has", b)
		c.callbackStore.Delete(session)
	}
	return
}

func (c *connImpl) recvFailed(msg *Message) {
	trigger, b := c.getCallback(msg.Session)
	if b {
		trigger(msg)
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
