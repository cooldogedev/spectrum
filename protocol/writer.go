package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/cooldogedev/spectrum/internal"
)

type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(data []byte) (err error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	if err = binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}

	buf.Write(data)
	if _, err := w.w.Write(buf.Bytes()); err != nil {
		return err
	}
	return
}
