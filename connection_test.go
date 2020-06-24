package scdt

import (
	"fmt"
	"github.com/portmapping/go-reuse"
	"net"
	"testing"
	"time"
)

func TestListener_Stop(t *testing.T) {
	lis, err := NewListener("0.0.0.0:12345")
	if err != nil {
		panic(err)
	}
	time.Sleep(30 * time.Hour)
	lis.Stop()

}
func TestConnImpl_MessageCallback(t *testing.T) {
	addr := net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	for i := 0; i < 1; i++ {
		dial, err := reuse.Dial("tcp", addr.String(), "localhost:12345")
		if err != nil {
			t.Fatal(err)
		}
		connect := Connect(dial)
		//connect.MessageCallback(func(data []byte) {
		//	fmt.Println(string(data))
		//})
		log.Infow("request remote id", "local", connect.LocalID())

		id, err := connect.RemoteID()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("local id", connect.LocalID(), "remote id", id)
	}
	time.Sleep(30 * time.Minute)
}
