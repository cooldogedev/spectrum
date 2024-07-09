package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// Latency is used for latency measurement between a client connected to the proxy and the server:
// 1. Proxy sends a Latency packet with the current timestamp and the client's ping.
// 2. Server measures the time taken for the packet to arrive and adds the client's latency.
// 3. Server responds with a Latency packet containing the total calculated latency.
type Latency struct {
	// Latency is the measured latency in milliseconds.
	Latency int64
	// Timestamp is the timestamp (in milliseconds) when the latency measurement was sent.
	Timestamp int64
}

// ID ...
func (pk *Latency) ID() uint32 {
	return IDLatency
}

// Marshal ...
func (pk *Latency) Marshal(io protocol.IO) {
	io.Int64(&pk.Latency)
	io.Int64(&pk.Timestamp)
}
