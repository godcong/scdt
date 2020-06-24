package scdt

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/portmapping/go-reuse"
)

func TestListener_Stop(t *testing.T) {
	lis, err := NewListener("0.0.0.0:12345")
	if err != nil {
		panic(err)
	}

	ip := "0.0.0.0:6060"
	if err := http.ListenAndServe(ip, nil); err != nil {
		fmt.Printf("start pprof failed on %s\n", ip)
	}
	lis.Stop()

}
func TestConnImpl_MessageCallback(t *testing.T) {
	addr := net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	wg := &sync.WaitGroup{}
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dial, err := reuse.Dial("tcp", addr.String(), "localhost:12345")
			if err != nil {
				log.Errorw("dail error", "err", err)
			}
			connect := Connect(dial)
			log.Infow("request remote id", "local", connect.LocalID())
			id, err := connect.RemoteID()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("local id", connect.LocalID(), "remote id", id)
		}()
	}
	wg.Wait()
}
