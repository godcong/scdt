package scdt

import (
	"context"
	"net"
	"runtime"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/portmapping/go-reuse"
)

type listener struct {
	ctx      context.Context
	cancel   context.CancelFunc
	listener net.Listener
	pool     *ants.Pool
	gcTicker *time.Ticker
}

func (l *listener) Stop() error {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	return nil
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
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			continue
		}
		l.pool.Submit(func() {
			c := Accept(conn)
			c.Wait()
		})
	}
}

func NewListener(addr string) (Listener, error) {
	l, err := reuse.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	pool, err := ants.NewPool(ants.DefaultAntsPoolSize, ants.WithNonblocking(false))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.TODO())
	lis := &listener{
		ctx:      ctx,
		cancel:   cancel,
		listener: l,
		pool:     pool,
		gcTicker: time.NewTicker(30 * time.Minute),
	}

	go lis.gc()
	go lis.listen()
	return lis, nil

}
