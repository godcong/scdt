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
	closed        chan bool
}

func (c *connImpl) LocalID() string {
	return c.localID
}

func (c *connImpl) RemoteID() (string, error) {
	id := c.remoteID.Load()
	if id == "" {
		queue := CallbackQueue(NewSendMessage(MessageConnectID, nil))
		if queue.Send(c.sendQueue) {
			msg := queue.Wait()
			if msg != nil && msg.DataLength != 0 {
				id = string(msg.Data)
				c.remoteID.Store(id)
			}
		}
	}
	return id, nil
}

func (c *connImpl) MessageCallback(fn MessageCallbackFunc) {
	c.fn = fn
}

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
		closed:        make(chan bool),
	}
	return runConnection(impl)
}

// AcceptNode ...
func Accept(conn net.Conn, cfs ...ConfigFunc) Connection {
	return NewConn(conn, cfs...)
}

// ConnectNode ...
func Connect(conn net.Conn, cfs ...ConfigFunc) Connection {
	cfs = append(cfs, func(c *Config) {
		c.HearBeatCheck = true
	})
	return NewConn(conn, cfs...)
}
func (c *connImpl) Wait() {
	<-c.closed
	close(c.closed)
	c.closed = nil
}

func (c *connImpl) sendMessage(pack WritePacker) error {
	return pack.Pack(c.conn)
}

func (c *connImpl) recv() {
	scan := dataScan(c.conn)
	for scan.Scan() {
		log.Infow("recv running")
		select {
		case <-c.ctx.Done():
			return
		default:
			var msg Message
			err := ScanExchange(scan, &msg)
			if err != nil {
				panic(err)
			}
			log.Infow("recv", "msg", msg)
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
			if !c.cfg.HearBeatCheck {
				continue
			}
			err = c.sendMessage(NewSendMessage(MessageHeartBeat, nil))
			if err != nil {
				panic(err)
			}
			if c.cfg.Timeout > 0 {
				c.hbCheck.Reset(c.cfg.Timeout)
			}

		case q := <-c.sendQueue:
			log.Infow("send", "msg", q.message)
			c.addCallback(q)
			err = c.sendMessage(q.message)
			if err != nil {
				panic(err)
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
	log.Infow("close")
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.hbCheck.Reset(0)

	if c.closed != nil {
		c.closed <- true
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
	MessageDataTransfer: recvRequestDataTransfer,
	MessageHeartBeat:    recvRequestHearBeat,
	MessageConnectID:    recvRequestID,
	MessageUserCustom:   recvRequest,
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
func recvRequestID(src *Message, v interface{}) (msg *Message, err error) {
	id := v.(string)
	log.Infow("local", "id", id)
	msg = NewRecvMessage(src.MessageID)
	msg.SetDataString(id)
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
