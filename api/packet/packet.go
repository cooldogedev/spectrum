package packet

import "bytes"

// Packet ...
type Packet interface {
	ID() uint32
	Encode(buf *bytes.Buffer)
	Decode(buf *bytes.Buffer)
}
