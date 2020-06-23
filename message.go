package scdt

type RequestType uint32
type MessageProcess uint32
type RequestStatus uint32

type Message struct {
	Version  Version
	Length   uint64
	Session  uint32
	Type     RequestType
	Process  MessageProcess
	Status   RequestStatus
	CheckSum []byte
	Data     []byte
}
