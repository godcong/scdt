package scdt

import (
	"encoding/binary"
	"io"
)

type RequestType uint32    //4
type MessageProcess uint32 //4
type RequestStatus uint32  //4
type ProcessSession uint32 //4
type DataLength uint64     //8
type SumLength uint16      //2
type Extension uint16      //2
// Version //4

type Message struct {
	Version    Version
	SumLength  SumLength
	DataLength DataLength
	Session    ProcessSession
	Type       RequestType
	Process    MessageProcess
	Status     RequestStatus
	Extension  Extension
	CheckSum   []byte
	Data       []byte
}

func (m *Message) Unpack(reader io.Reader) (err error) {
	var v []interface{}
	v = append(v, &m.Version, &m.DataLength, &m.SumLength, &m.Session, &m.Type, &m.Process, &m.Status, &m.Extension)
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

	if m.SumLength != 0 {
		m.CheckSum = make([]byte, m.SumLength)
		return binary.Read(reader, binary.BigEndian, &m.CheckSum)
	}

	return nil
}

// Pack ...
func (m Message) Pack(writer io.Writer) (err error) {
	var v []interface{}
	v = append(v, &m.Version, &m.DataLength, &m.SumLength, &m.Session, &m.Type, &m.Process, &m.Status, &m.Extension, &m.CheckSum, &m.Data)
	for i := range v {
		err = binary.Write(writer, binary.BigEndian, v[i])
		if err != nil {
			return err
		}
	}
	return nil
}
