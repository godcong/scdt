package scdt

type Message struct {
	Version    Version
	Length     uint64
	Session    uint32
	Type       Type
	TypeDetail TypeDetail
	Status     Status
	Data       []byte
}
