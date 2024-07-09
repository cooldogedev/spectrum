package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Reader is used for reading packets from an io.Reader.
type Reader struct {
	// r is the underlying io.Reader used for reading data.
	r io.Reader
}

// NewReader creates a new Reader with the given io.Reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// ReadPacket reads a packet from the underlying io.Reader.
// It first reads the length of the packet as an uint32 in big-endian order,
// then reads the actual packet data of that length.
func (r *Reader) ReadPacket() ([]byte, error) {
	var length uint32
	if err := binary.Read(r.r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read packet length: %w", err)
	}

	pk := make([]byte, length)
	if _, err := io.ReadFull(r.r, pk); err != nil {
		return nil, fmt.Errorf("failed to read packet data: %w", err)
	}
	return pk, nil
}
