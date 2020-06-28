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
		return nil, errors.New("connection id was unsupported")
	}
	connection, b := load.(Connection)
	if b {
		return nil, errors.New("failed transfer to Connection")
	}
	return connection, nil
}

// SendTo ...
func (l *listener) SendCustomTo(id string, cid CustomID, data []byte, f func(id string, message *Message)) bool {
	conn, err := l.getConn(id)
	if err != nil {
		return false
	}
	return conn.SendCustomDataWithCallback(cid, data, func(message *Message) {
		if f != nil {
			f(id, message)
		}
	})
}

// SendTo ...
func (l *listener) SendTo(id string, data []byte, f func(id string, message *Message)) bool {
	conn, err := l.getConn(id)
	if err != nil {
		return false
	}
	return conn.SendWithCallback(data, func(message *Message) {
		if f != nil {
			f(id, message)
		}
	})
}

// HandleRecv ...
func (l *listener) HandleRecv(fn HandleRecvFunc) {
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

func (l *listener) listen() {
	myid := UUID()
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			continue
		}
		l.pool.Submit(func() {
			c := Accept(conn, func(c *Config) {
				c.CustomIDer = func() string {
					return myid
				}
			})
			id, err := c.RemoteID()
			if err != nil {
				return
			}
			c.Recv(func(message *Message) ([]byte, bool) {
				if l.recvFunc != nil {
					return l.recvFunc(id, message)
				}
				return nil, false
			})
			l.conns.Store(id, c)
			for !c.IsClosed() {
				time.Sleep(15 * time.Second)
				log.Infow("connecting", "id", id)
			}
			l.conns.Delete(id)
		})
	}
}

// NewListener ...
func NewListener(addr string) (Listener, error) {
	l, err := reuse.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	pool, poolErr := ants.NewPool(ants.DefaultAntsPoolSize, ants.WithNonblocking(false))
	if poolErr != nil {
		return nil, poolErr
	}
	ctx, cancel := context.WithCancel(context.TODO())
	lis := &listener{
		ctx:      ctx,
		cancel:   cancel,
		listener: l,
		pool:     pool,
		conns:    new(sync.Map),
		gcTicker: defaultGCTimer,
	}

	go lis.gc()
	go lis.listen()
	return lis, nil

}
