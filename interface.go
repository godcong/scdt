package scdt

import (
	"io"
	"net"
)

// MessageCallbackFunc ...
type MessageCallbackFunc func(data []byte)

// Listener ...
type Listener interface {
	Stop() error
	HandleRecv(fn HandleRecvFunc)
	SendCustomTo(id string, cid CustomID, data []byte, f func(id string, message *Message)) (*Queue, bool)
	SendTo(id string, data []byte, f func(id string, message *Message)) (*Queue, bool)
	Range(f func(id string, connection Connection))
}

// Connection ...
type Connection interface {
	LocalID() string
	RemoteID() (string, error)
	Ping() (string, error)
	Close()
	IsClosed() bool
	NetConn() net.Conn
	SendQueue(q *Queue) bool
	Recv(fn RecvCallbackFunc)
	OnRecv(fn OnRecvCallbackFunc)
	RecvCustomData(fn RecvCallbackFunc)
	SendCustomData(id CustomID, data []byte) (*Queue, bool)
	SendCustomDataOnWait(id CustomID, data []byte) (msg *Message, b bool)
	SendCustomDataWithCallback(id CustomID, data []byte, cb func(message *Message)) (*Queue, bool)
	Send(data []byte) (*Queue, bool)
	SendOnWait(data []byte) (*Message, bool)
	SendWithCallback(data []byte, cb func(message *Message)) (*Queue, bool)
	SendClose([]byte) bool
}

// SendCallback ...
type SendCallback func(packer ReadPacker)

// ReadPacker ...
type ReadPacker interface {
	Unpack(reader io.Reader) (err error)
}

// WritePacker ...
type WritePacker interface {
	Pack(writer io.Writer) (err error)
}

// ReadWritePacker ...
type ReadWritePacker interface {
	ReadPacker
	WritePacker
}
