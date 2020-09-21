package acserver

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode/utf32"
)

type Packet struct {
	buf *bytes.Buffer
}

var (
	utf32Encoder = utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewEncoder()
	utf32Decoder = utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewDecoder()
)

func NewPacket(b []byte) *Packet {
	return &Packet{
		buf: bytes.NewBuffer(b),
	}
}

func (p *Packet) Write(val interface{}) {
	err := binary.Write(p.buf, binary.LittleEndian, val)

	if err != nil {
		logrus.WithError(err).Errorf("Could not Write: %v", val)
	}
}

func (p *Packet) WriteString(s string) {
	p.Write(uint8(len(s)))
	p.Write([]byte(s))
}

func (p *Packet) WriteUTF32String(s string) {
	encoded, err := utf32Encoder.Bytes([]byte(s))

	if err != nil {
		logrus.WithError(err).Error("Could not EncodeString")
	}
	p.Write(uint8(len([]rune(s))))
	p.Write(encoded)
}

func (p *Packet) WriteBigUTF32String(s string) {
	s += "\x00"

	encoded, err := utf32Encoder.Bytes([]byte(s))

	if err != nil {
		logrus.WithError(err).Error("Could not EncodeString")
	}

	p.Write(uint16(len([]rune(s))))
	p.Write(encoded)
}

func (p *Packet) Read(out interface{}) {
	_ = binary.Read(p.buf, binary.LittleEndian, out)
}

func (p *Packet) ReadUint8() uint8 {
	var i uint8

	p.Read(&i)

	return i
}

func (p *Packet) ReadString() string {
	size := p.ReadUint8()

	b := make([]byte, size)

	p.Read(&b)

	return string(b)
}

func (p *Packet) ReadUTF32String() string {
	size := p.ReadUint8()

	b := make([]byte, int(size)*4)

	p.Read(&b)

	bs, _ := utf32Decoder.Bytes(b)

	return string(bs)
}

func (p *Packet) ReadUint16() uint16 {
	var i uint16

	p.Read(&i)

	return i
}

func (p *Packet) ReadUint32() uint32 {
	var i uint32

	p.Read(&i)

	return i
}

func (p *Packet) ReadInt16() int16 {
	var i int16

	p.Read(&i)

	return i
}

func (p *Packet) ReadCarID() CarID {
	return CarID(p.ReadUint8())
}

func (p *Packet) WriteTCP(w io.Writer) error {
	if w == nil {
		return nil
	}

	out := make([]byte, 2)

	b := p.buf.Bytes()

	binary.LittleEndian.PutUint16(out, uint16(len(b)))
	out = append(out, b...)

	_, err := w.Write(out)
	return err
}

type writerTo interface {
	WriteTo(b []byte, addr net.Addr) (int, error)
}

func (p *Packet) WriteUDP(conn writerTo, addr net.Addr) error {
	if addr == nil {
		return nil
	}

	b := p.buf.Bytes()

	_, err := conn.WriteTo(b, addr)

	return err
}

func (p *Packet) WriteToUDPConn(conn *net.UDPConn) error {
	_, err := conn.Write(p.buf.Bytes())

	return err
}
