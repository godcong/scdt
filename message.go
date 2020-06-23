package scdt

type RequestType uint32
type MessageProcess uint32
type RequestStatus uint32
type ProcessSession uint32
type MessageLength uint64

type Message struct {
	Version  Version
	Length   MessageLength
	Session  ProcessSession
	Type     RequestType
	Process  MessageProcess
	Status   RequestStatus
	CheckSum []byte
	Data     []byte
}
