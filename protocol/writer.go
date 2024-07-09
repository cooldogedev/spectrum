package protocol

import (
	"encoding/binary"
	"io"
)

// Writer is used for writing packets to an io.Writer.
type Writer struct {
	// w is the underlying io.Writer used for writing data.
	w io.Writer
}

// NewWriter creates a new Writer with the given io.Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write writes a packet to the underlying io.Writer.
// It first writes the length of the packet as an uint32 in big-endian order,
// followed by the actual packet data itself.
func (w *Writer) Write(data []byte) (err error) {
	if err = binary.Write(w.w, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}

	if _, err := w.w.Write(data); err != nil {
		return err
	}
	return
}
