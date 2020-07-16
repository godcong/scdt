package scdt

import (
	"encoding/binary"
	"errors"
	"io"
)

// RequestType ...
type RequestType uint8

// MessageID ...
type MessageID uint8

// CustomID ...
type CustomID uint16

// RequestStatus ...
type RequestStatus uint32

// Session ...
type Session uint32

// DataLength ...
type DataLength uint64

// SumLength ...
type SumLength uint16

// Extension ...
type Extension uint16

// Version //4
const (
	// RequestTypeRecv ...
	RequestTypeRecv RequestType = iota + 1
	// RequestTypeSend ...
	RequestTypeSend
	// RequestTypeFailed ...
	RequestTypeFailed
	// RequestTypeClose ...
	RequestTypeClose
)

const (
	// MessagePing ...
	MessagePing MessageID = iota + 1
	// MessageHeartBeat ...
	MessageHeartBeat
	// MessageConnectID ...
	MessageConnectID
	// MessageDataTransfer ...
	MessageDataTransfer
	// MessageUserCustom ...
	MessageUserCustom
	// MessageRecvFailed ...
	MessageRecvFailed
	// MessageClose ...
	MessageClose
)

// Message ...
type Message struct {
	version     Version
	requestType RequestType
	MessageID   MessageID
	CustomID    CustomID
	DataLength  DataLength
	Session     Session
	Data        []byte
}

// newSendMessage ...
func newSendMessage(id MessageID, data []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeSend,
		MessageID:   id,
		DataLength:  Length(data),
		Data:        data,
	}
}

// newRecvMessage ...
func newRecvMessage(id MessageID) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeRecv,
		MessageID:   id,
	}
}

func newCloseMessage(id MessageID, msg []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeClose,
		MessageID:   id,
		DataLength:  Length(msg),
		Data:        msg,
	}
}

// newCustomSendMessage ...
func newCustomSendMessage(id CustomID, data []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeSend,
		MessageID:   MessageUserCustom,
		CustomID:    id,
		Data:        data,
		DataLength:  Length(data),
	}
}

// newCustomRecvMessage ...
func newCustomRecvMessage(id CustomID, data []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeRecv,
		MessageID:   MessageUserCustom,
		CustomID:    id,
		Data:        data,
		DataLength:  Length(data),
	}
}

func newFailedSendMessage(data []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeFailed,
		MessageID:   MessageRecvFailed,
		Data:        data,
		DataLength:  Length(data),
	}
}

// Unpack ...
func (m *Message) Unpack(reader io.Reader) (err error) {
	var v []interface{}
	v = append(v, &m.version, &m.requestType, &m.MessageID, &m.CustomID, &m.DataLength, &m.Session)
	for i := range v {
		err = binary.Read(reader, binary.BigEndian, v[i])
		if err != nil {
			return err
		}
	}

	if m.DataLength != 0 {
		m.Data = make([]byte, m.DataLength)
		return binary.Read(reader, binary.BigEndian, &m.Data)
	}

	return nil
}

// Pack ...
func (m Message) Pack(writer io.Writer) (err error) {
	var v []interface{}
	v = append(v, &m.version, &m.requestType, &m.MessageID, &m.CustomID, &m.DataLength, &m.Session, &m.Data)
	for i := range v {
		err = binary.Write(writer, binary.BigEndian, v[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// SetCustomID ...
func (m *Message) SetCustomID(id CustomID) *Message {
	m.MessageID = MessageUserCustom
	m.CustomID = id
	return m
}

// SetDataString ...
func (m *Message) SetDataString(data string) *Message {
	m.Data = []byte(data)
	m.DataLength = Length(m.Data)
	return m
}

// SetData ...
func (m *Message) SetData(data []byte) *Message {
	m.Data = data
	m.DataLength = Length(m.Data)
	return m
}

// RequestType ...
func (m *Message) RequestType() RequestType {
	return m.requestType
}

// SetRequestType ...
func (m *Message) SetRequestType(requestType RequestType) *Message {
	m.requestType = requestType
	return m
}

// Error ...
func (m *Message) Error() error {
	if m.requestType != RequestTypeFailed {
		return nil
	}
	return errors.New(string(m.Data))
}

// Length ...
func Length(data []byte) DataLength {
	return DataLength(len(data))
}
