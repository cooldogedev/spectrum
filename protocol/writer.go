package protocol

import (
	"encoding/binary"
	"io"
)

type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(data []byte) (err error) {
	if err = binary.Write(w.w, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}

	if _, err := w.w.Write(data); err != nil {
		return err
	}
	return
}
