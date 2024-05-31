package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Reader struct {
	r io.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

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
