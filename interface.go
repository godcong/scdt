package scdt

import "io"

type MessageCallbackFunc func(data []byte)

type Listener interface {
	Stop() error
}

type Connection interface {
	LocalID() string
	RemoteID() (string, error)
	MessageCallback(fn MessageCallbackFunc)
	Close()
	Wait()
}

type SendCallback func(packer ReadPacker)

type ReadPacker interface {
	Unpack(reader io.Reader) (err error)
}

type WritePacker interface {
	Pack(writer io.Writer) (err error)
}

type ReadWritePacker interface {
	ReadPacker
	WritePacker
}
