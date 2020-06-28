package scdt

import "io"

// MessageCallbackFunc ...
type MessageCallbackFunc func(data []byte)

// Listener ...
type Listener interface {
	Stop() error
}

// Connection ...
type Connection interface {
	LocalID() string
	RemoteID() (string, error)
	MessageCallback(fn MessageCallbackFunc)
	Close()
	IsClosed() bool
	Recv(fn RecvCallbackFunc)
	SendCustomData(id CustomID, data []byte) bool
	SendCustomDataOnWait(id CustomID, data []byte) (msg *Message, b bool)
	SendCustomDataWithCallback(id CustomID, data []byte, cb func(message *Message)) bool
	Send(data []byte) bool
	SendOnWait(data []byte) (*Message, bool)
	SendWithCallback(data []byte, cb func(message *Message)) bool
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
