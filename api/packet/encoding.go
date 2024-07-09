package packet

import (
	"bytes"
	"encoding/binary"
)

// ReadString reads a string from buf, where the string is prefixed with its length
// encoded as an uint32 in little-endian order.
func ReadString(buf *bytes.Buffer) string {
	var length uint32
	_ = binary.Read(buf, binary.LittleEndian, &length)
	data := make([]byte, length)
	_, _ = buf.Read(data)
	return string(data)
}

// WriteString writes the string s to buf, prefixing it with its length encoded
// as an uint32 in little-endian order.
func WriteString(buf *bytes.Buffer, s string) {
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(s)))
	buf.Write([]byte(s))
}
