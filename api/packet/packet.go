package packet

import "bytes"

// Packet represents a protocol packet that can be sent over an API connection.
// It defines methods for identifying the packet, encoding itself to binary,
// and decoding itself from binary.
type Packet interface {
	// ID returns the unique identifier of the packet.
	ID() uint32
	// Encode will encode the packet into binary form and write it to buf.
	Encode(buf *bytes.Buffer)
	// Decode will decode binary data from buf into the packet.
	Decode(buf *bytes.Buffer)
}
