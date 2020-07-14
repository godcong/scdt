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
type RecvCallbackFunc func(message *Message) ([]byte, bool, error)

var defaultConnSendTimeout = 30 * time.Second
var recvReqFunc = map[MessageID]func(src *Message, v interface{}) (msg *Message, b bool, err error){
	MessageHeartBeat: recvRequestHearBeat,
	MessageConnectID: recvRequestID,
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
		//msg := newRecvMessage(MessageConnectID)
		//msg.SetDataString("hello world")
		queue := CallbackQueue(newRecvMessage(MessageConnectID))
		if queue.send(c.sendQueue) {
			msg := queue.Wait()
			log.Debugw("result msg", "msg", msg)
			if msg != nil && msg.DataLength != 0 {
				log.Debugw("debug data detail", "data", string(msg.Data))
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
			log.Debugw("recv", "msg", msg, "msgRequest", msg.RequestType(), "data", msg.Data)
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
			log.Debugw("send", "msg", q.message, "msgRequest", q.message.RequestType())
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
		b = msg != nil
	}
	return
}

// send ...
func (c *connImpl) SendCustomData(id CustomID, data []byte) (*Queue, bool) {
	return c.SendCustomDataWithCallback(id, data, nil)
}

// send ...
func (c *connImpl) SendCustomDataWithCallback(id CustomID, data []byte, cb func(message *Message)) (*Queue, bool) {
	queue := CallbackQueue(newCustomRecvMessage(id, data)).SetRecvCallback(cb)
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
		b = msg != nil
	}
	return
}

// send ...
func (c *connImpl) SendWithCallback(data []byte, cb func(message *Message)) (*Queue, bool) {
	queue := CallbackQueue(newRecvMessage(MessageDataTransfer).SetData(data)).SetRecvCallback(cb)
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

// SendClose ...
func (c *connImpl) SendClose(msg []byte) bool {
	return CallbackQueue(newCloseMessage(MessageClose, msg)).send(c.sendQueue)
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
	case RequestTypeClose:
		c.recvClose(msg)
	default:
		panic("unsupported request type")
		//return
	}
}

func recvRequestDataTransfer(src *Message, v RecvCallbackFunc) (msg *Message, b bool, err error) {
	if v == nil {
		return newSendMessage(src.MessageID, nil), true, nil
	}

	var msgCopy Message
	msgCopy = *src
	copy(msgCopy.Data, src.Data)
	data, b, err := v(&msgCopy)
	log.Debugw("copy message info", "msg", msgCopy, "need", b)
	if !b {
		return &Message{}, false, errors.New("do not need response")
	}
	if err != nil {
		return &Message{}, true, err
	}
	return newSendMessage(src.MessageID, data), true, nil
}
func recvCustomRequest(src *Message, v RecvCallbackFunc) (msg *Message, b bool, err error) {
	if v == nil {
		return newCustomSendMessage(src.CustomID, nil), true, nil
	}
	var msgCopy Message
	msgCopy = *src
	copy(msgCopy.Data, src.Data)
	data, b, err := v(&msgCopy)
	log.Debugw("copy custom message info", "msg", msgCopy, "need", b)
	if !b {
		return &Message{}, false, errors.New("do not need response")
	}
	if err != nil {
		return &Message{}, true, err
	}
	return newCustomSendMessage(src.CustomID, data), true, nil
}
func recvRequestFailed(src *Message, v interface{}) (msg *Message, b bool, err error) {
	return newFailedSendMessage(toBytes(v.(string))), true, nil
}
func recvRequestHearBeat(src *Message, v interface{}) (msg *Message, b bool, err error) {
	return newSendMessage(src.MessageID, nil), true, nil
}
func recvRequestID(src *Message, v interface{}) (msg *Message, b bool, err error) {
	id := v.(string)
	return newSendMessage(src.MessageID, toBytes(id)), true, nil
}

func (c *connImpl) getMessageArgs(id MessageID) interface{} {
	switch id {
	case MessageConnectID:
		return c.localID
	}
	return nil
}

func (c *connImpl) recvRequest(msg *Message) {
	f, b := recvReqFunc[msg.MessageID]
	var newMsg *Message
	var err error
	if b {
		newMsg, b, err = f(msg, c.getMessageArgs(msg.MessageID))
	} else if msg.MessageID == MessageDataTransfer {
		newMsg, b, err = recvRequestDataTransfer(msg, c.recvCallback)
	} else if msg.MessageID == MessageUserCustom {
		newMsg, b, err = recvCustomRequest(msg, c.recvCustomDataCallback)
	} else {
		newMsg, b, _ = recvRequestFailed(msg, "no case matched")
	}
	log.Debugw("received", "msg", newMsg, "result", b, "err", err)
	if !b {
		return
	}

	if err != nil {
		//ignore err, ignore result tag
		newMsg, _, _ = recvRequestFailed(msg, err.Error())
	}
	newMsg.MessageID = msg.MessageID
	newMsg.Session = msg.Session
	newMsg.CustomID = msg.CustomID
	if newMsg.DataLength != 0 {
		log.Debugw("received data", "type", newMsg.RequestType(), "data", string(newMsg.Data))
	}
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
	log.Debugw("load callback", "session", session)
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
	c.Close()
}

func (c *connImpl) recvClose(msg *Message) {
	c.callbackStore.Range(func(key, value interface{}) bool {
		f, b := value.(func(message *Message))
		if b {
			f(msg)
		}
		return true
	})
	c.Close()
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
