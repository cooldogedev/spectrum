package protocol

import (
	"bytes"
	"encoding/binary"
	"github.com/cooldogedev/spectrum/internal"
)

type writable interface {
	Write([]byte) (int, error)
}

type Writer struct {
	w writable
}

func NewWriter(w writable) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(data []byte) (err error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	if err = binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return
	}
	buf.Write(data)
	_, err = w.w.Write(buf.Bytes())
	return
}
