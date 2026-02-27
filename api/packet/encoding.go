package packet

import (
	"bytes"
	"encoding/binary"
)

const maxAPIStringLength = 1024 * 1024

// ReadString reads a string from buf, where the string is prefixed with its length
// encoded as an uint32 in little-endian order.
func ReadString(buf *bytes.Buffer) string {
	var length uint32
	if err := binary.Read(buf, binary.LittleEndian, &length); err != nil {
		return ""
	}
	if length > maxAPIStringLength || int(length) > buf.Len() {
		return ""
	}
	data := make([]byte, length)
	_, _ = buf.Read(data)
	return string(data)
}

// WriteString writes the string s to buf, prefixing it with its length encoded
// as an uint32 in little-endian order.
func WriteString(buf *bytes.Buffer, s string) {
	if len(s) > maxAPIStringLength {
		s = s[:maxAPIStringLength]
	}
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(s)))
	buf.Write([]byte(s))
}
