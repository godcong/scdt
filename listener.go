package scdt

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/portmapping/go-reuse"
)

// HandleRecvFunc ...
type HandleRecvFunc func(id string, message *Message) ([]byte, bool)

type listener struct {
	ctx      context.Context
	cancel   context.CancelFunc
	listener net.Listener
	cfg      *Config
	id       string
	pool     *ants.Pool
	gcTicker *time.Ticker
	conns    *sync.Map
	recvFunc HandleRecvFunc
}

var defaultGCTimer = time.NewTicker(30 * time.Second)

// Stop ...
func (l *listener) Stop() error {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	return nil
}

func (l *listener) getConn(id string) (Connection, error) {
	load, ok := l.conns.Load(id)
	if !ok {
		return nil, errors.New("connection was not found")
	}
	connection, b := load.(Connection)
	if b {
		return nil, errors.New("failed transfer to Connection")
	}
	return connection, nil
}

// RangeConnections ...
func (l *listener) Range(f func(id string, connection Connection)) {
	l.conns.Range(func(key, value interface{}) bool {
		connection, valueB := value.(Connection)
		id, keyB := key.(string)
		if valueB && keyB {
			f(id, connection)
		}
		return false
	})
}

// SendTo ...
func (l *listener) SendCustomTo(id string, cid CustomID, data []byte, f func(id string, message *Message)) (*Queue, bool) {
	conn, err := l.getConn(id)
	if err != nil {
		return nil, false
	}
	return conn.SendCustomDataWithCallback(cid, data, func(message *Message) {
		if f != nil {
			f(id, message)
		}
	})
}

// SendTo ...
func (l *listener) SendTo(id string, data []byte, f func(id string, message *Message)) (*Queue, bool) {
	conn, err := l.getConn(id)
	if err != nil {
		return nil, false
	}
	return conn.SendWithCallback(data, func(message *Message) {
		if f != nil {
			f(id, message)
		}
	})
}

// HandleRecv ...
func (l *listener) HandleRecv(fn HandleRecvFunc) {
	log.Debugw("register recv", "funcNull", fn == nil)
	l.recvFunc = fn
}

func (l *listener) gc() {
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-l.gcTicker.C:
			running := l.pool.Running()
			if running == 0 {
				runtime.GC()
			}
		}
	}
}

func (l *listener) handleRecv(id string) func(message *Message) ([]byte, bool) {
	return func(message *Message) ([]byte, bool) {
		log.Debugw("recvFunc", "id", id, "message", message, "funcNull", l.recvFunc == nil)
		if l.recvFunc != nil {
			return l.recvFunc(id, message)
		}
		return nil, false
	}
}

func (l *listener) listen() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			continue
		}
		l.pool.Submit(func() {
			c := Accept(l.id, conn)
			id, err := c.RemoteID()
			if err != nil {
				return
			}
			log.Debugw("recieved id", "id", id)
			c.Recv(func(message *Message) ([]byte, bool, error) {
				remoteID, err := c.RemoteID()
				if err != nil {
					return nil, true, err
				}

				bytes, b := l.handleRecv(remoteID)(message)
				log.Debugw("recv message", "id", id, "message", message, "result data", string(bytes), "need", b)
				return bytes, b, nil
			})
			c.RecvCustomData(func(message *Message) ([]byte, bool, error) {
				remoteID, err := c.RemoteID()
				if err != nil {
					return nil, true, err
				}
				bytes, b := l.handleRecv(remoteID)(message)
				log.Debugw("recv message", "id", id, "message", message, "result data", string(bytes), "need", b)
				return bytes, b, nil
			})

			l.conns.Store(id, c)
			for !c.IsClosed() {
				time.Sleep(15 * time.Second)
				log.Debugw("connected from", "id", id)
			}
			l.conns.Delete(id)
		})
	}
}

// NewListener ...
func NewListener(id, addr string, cfs ...ConfigFunc) (Listener, error) {
	l, err := reuse.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	pool, poolErr := ants.NewPool(ants.DefaultAntsPoolSize, ants.WithNonblocking(false))
	if poolErr != nil {
		return nil, poolErr
	}
	ctx, cancel := context.WithCancel(context.TODO())
	cfg := defaultConfig()
	for _, cf := range cfs {
		cf(cfg)
	}
	lis := &listener{
		ctx:      ctx,
		cancel:   cancel,
		cfg:      cfg,
		id:       id,
		listener: l,
		pool:     pool,
		conns:    new(sync.Map),
		gcTicker: defaultGCTimer,
	}

	go lis.gc()
	go lis.listen()
	return lis, nil

}
