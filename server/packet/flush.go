package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// Flush signals the proxy to flush the client's buffer.
// Servers that implement a batching mechanism, such as PMMP, send this packet at the
// end of a batch to indicate that flushing should occur. This prevents the proxy from
// performing double batching, which would otherwise result in unnecessary delays for
// flushing (the default delay is 50ms).
type Flush struct {
}

// ID ...
func (pk *Flush) ID() uint32 {
	return IDFlush
}

// Marshal ...
func (pk *Flush) Marshal(_ protocol.IO) {
}
