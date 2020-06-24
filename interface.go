package scdt

import "io"

type Listener interface {
	MessageCallback(func(data []byte))
}

type Connection interface {
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
