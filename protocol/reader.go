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

func (r *Reader) ReadPacket() ([]byte, error) {
	select {
	case packet := <-r.packets:
		return packet, nil
	default:
		if err := r.read(); err != nil {
			return nil, err
		}
		return r.ReadPacket()
	}
}

func (r *Reader) read() (err error) {
	if r.remaining <= 0 {
		lengthBytes, err := r.internalRead(uint32(packetLengthSize - len(r.buf)))
		if err != nil {
			return err
		}

		r.buf = append(r.buf, lengthBytes...)
		if len(r.buf) < 4 {
			return nil
		}

		r.remaining = binary.BigEndian.Uint32(r.buf)
		r.buf = nil
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

	pk := r.buf
	r.buf = nil
	r.remaining = 0
	r.packets <- pk
	return
}

func (r *Reader) internalRead(n uint32) ([]byte, error) {
	if n > packetFrameSize {
		n = packetFrameSize
	}
	data := make([]byte, n)
	_, err := r.r.Read(data)
	return data, err
}
