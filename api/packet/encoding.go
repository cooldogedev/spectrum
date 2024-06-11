package packet

import (
	"bytes"
	"encoding/binary"
)

func ReadString(buf *bytes.Buffer) string {
	var length uint32
	_ = binary.Read(buf, binary.LittleEndian, &length)
	data := make([]byte, length)
	_, _ = buf.Read(data)
	return string(data)
}

func WriteString(buf *bytes.Buffer, s string) {
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(s)))
	buf.Write([]byte(s))
}
