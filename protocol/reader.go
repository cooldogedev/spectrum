package protocol

import (
	"encoding/binary"
	"io"
)

const (
	packetLengthSize = 4
	packetFrameSize  = 1024 * 64
)

type Reader struct {
	r         io.Reader
	buf       []byte
	remaining uint32
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

func (r *Reader) ReadPacket() ([]byte, error) {
	if r.remaining <= 0 && r.buf != nil {
		pk := r.buf
		r.buf = nil
		return pk, nil
	}

	if err := r.read(); err != nil {
		return nil, err
	}
	return r.ReadPacket()
}

func (r *Reader) read() error {
	if r.remaining <= 0 {
		lengthBytes, err := r.readBytes(uint32(packetLengthSize - len(r.buf)))
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

	data, err := r.readBytes(r.remaining)
	if err != nil {
		return err
	}

	r.buf = append(r.buf, data...)
	r.remaining -= uint32(len(data))
	return nil
}

func (r *Reader) readBytes(n uint32) ([]byte, error) {
	if n > packetFrameSize {
		n = packetFrameSize
	}

	data := make([]byte, n)
	if _, err := r.r.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}
