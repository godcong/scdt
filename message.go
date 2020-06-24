package scdt

import (
	"encoding/binary"
	"io"
)

type RequestType uint16   //2
type MessageID uint16     //2
type RequestStatus uint32 //4
type Session uint32       //4
type DataLength uint64    //8
type SumLength uint16     //2
type Extension uint16     //2

// Version //4
const (
	RequestTypeRecv   RequestType = 0x00
	RequestTypeSend   RequestType = 0x01
	RequestTypeFailed RequestType = 0x02
)

const (
	MessageHeartBeat MessageID = iota
	MessageConnectID
	MessageDataTransfer
	MessageUserCustom
	c
)

type Message struct {
	version     Version
	requestType RequestType
	MessageID   MessageID
	DataLength  DataLength
	Session     Session
	Data        []byte
}

func NewSendMessage(id MessageID, data []byte) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeSend,
		MessageID:   id,
		DataLength:  DataLength(len(data)),
		Data:        data,
	}
}

func NewRecvMessage(id MessageID) *Message {
	return &Message{
		version:     Version{'v', 0, 0, 1},
		requestType: RequestTypeRecv,
		MessageID:   id,
	}
}

func (m *Message) Unpack(reader io.Reader) (err error) {
	var v []interface{}
	v = append(v, &m.version, &m.requestType, &m.MessageID, &m.DataLength)
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
	v = append(v, &m.version, &m.requestType, &m.MessageID, &m.DataLength, &m.Data)
	for i := range v {
		err = binary.Write(writer, binary.BigEndian, v[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) SetDataString(data string) {
	m.Data = []byte(data)
	m.DataLength = DataLength(len(m.Data))
}

func (m *Message) SetData(data []byte) {
	m.Data = data
	m.DataLength = DataLength(len(m.Data))
}
