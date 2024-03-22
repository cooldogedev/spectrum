package internal

import (
	"bytes"
	"sync"
)

var BufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256))
	},
}
