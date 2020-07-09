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
	id := UUID()
	lis, err := NewListener("0.0.0.0:12345", func(c *Config) {
		c.CustomIDer = func() string {
			return id
		}
	})
	lis.HandleRecv(func(id string, message *Message) ([]byte, bool) {
		log.Infow("receive callback", "id", id, "message", message)
		return []byte("the world give you a data"), true
	})
	lis.SetGlobalID(func() string {
		return id
	})
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
	for i := 0; i < 2; i++ {
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
			_, b := connect.SendCustomDataWithCallback(0x01, []byte("hello send with callback"), func(message *Message) {
				fmt.Printf("send message:%+v,data:%s\n", message, message.Data)
			})
			//if b {
			//	wait := callback.Wait()
			//	if wait != nil {
			//		fmt.Printf("waited send message callback:%+v,data:%s\n", wait, wait.Data)
			//	}
			//}
			msg, b := connect.SendCustomDataOnWait(0x02, []byte("hello send on wait"))
			if b {
				fmt.Printf("waited send message:%+v,data:%s\n", msg, msg.Data)
			}
			//connect.SendCustomDataWithCallback(0x01, []byte("hello"), func(message *Message) {
			//
			//})
			//connect.RecvCustomData(func(message *Message) ([]byte, bool) {
			//	fmt.Printf("recv message:%+v,data:%s\n", id, message.Data)
			//	return nil, false
			//})
		}()
	}
	wg.Wait()
}
func TestConnImpl_LocalID(t *testing.T) {
	// MessageHeartBeat ...
	t.Log(MessageHeartBeat)
	// MessageConnectID ...
	t.Log(MessageConnectID)
	// MessageDataTransfer ...
	t.Log(MessageDataTransfer)
	// MessageUserCustom ...
	t.Log(MessageUserCustom)
	// MessageRecvFailed ...
	t.Log(MessageRecvFailed)

}
