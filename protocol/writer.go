package protocol

import (
	"encoding/binary"
	"io"
)

// Writer is used for writing packets to an io.Writer.
type Writer struct {
	// w is the underlying io.Writer used for writing data.
	w io.Writer
	// p is a reusable byte slice used for writing the length of the packet.
	p []byte
}

// NewWriter creates a new Writer with the given io.Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: w,
		p: make([]byte, 4),
	}
}

// Write writes a packet to the underlying io.Writer.
// It prefixes the packet data with its length as an uint32 in big-endian order,
// then writes the prefixed data to the underlying io.Writer.
func (w *Writer) Write(data []byte) (err error) {
	binary.BigEndian.PutUint32(w.p, uint32(len(data)))
	if _, err := w.w.Write(append(w.p, data...)); err != nil {
		return err
	}
	return
}
