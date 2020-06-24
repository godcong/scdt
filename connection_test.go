package scdt

import (
	"fmt"
	"github.com/portmapping/go-reuse"
	"testing"
	"time"
)

func servListen() {
	lis, err := NewListener("0.0.0.0:12345")
	if err != nil {
		panic(err)
	}
	time.Sleep(30 * time.Minute)
	lis.Stop()
}

func init() {
	go servListen()
}

func TestConnImpl_MessageCallback(t *testing.T) {
	dial, err := reuse.Dial("tcp", "", "localhost:12345")
	if err != nil {
		t.Fatal(err)
	}
	connect := Connect(dial)
	connect.MessageCallback(func(data []byte) {
		fmt.Println(string(data))
	})
}