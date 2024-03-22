package protocol

import (
	"encoding/binary"
)

const (
	packetLengthSize = 4
	packetFrameSize  = 1024 * 64
)

type readable interface {
	Read([]byte) (int, error)
}

type Reader struct {
	r         readable
	buf       []byte
	remaining uint32
	packets   chan []byte
}

func NewReader(r readable) *Reader {
	return &Reader{
		r:       r,
		packets: make(chan []byte, 10),
	}
}

func (r *Reader) Read() (err error) {
	if r.remaining <= 0 {
		length, err := r.internalRead(packetLengthSize)
		if err != nil {
			return err
		}
		r.remaining = binary.BigEndian.Uint32(length)
	}

	data, err := r.internalRead(r.remaining)
	if err != nil {
		return err
	}

	r.buf = append(r.buf, data...)
	r.remaining -= uint32(len(data))
	if r.remaining > 0 {
		return
	}

	r.packets <- r.buf
	r.buf = nil
	r.remaining = 0
	return
}

func (r *Reader) ReadPacket() []byte {
	select {
	case packet := <-r.packets:
		return packet
	}
}

func (r *Reader) internalRead(n uint32) ([]byte, error) {
	if n > packetFrameSize {
		n = packetFrameSize
	}
	data := make([]byte, n)
	_, err := r.r.Read(data)
	return data, err
}
