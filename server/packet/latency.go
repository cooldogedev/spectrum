package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type Latency struct {
	Latency   int64
	Timestamp int64
}

func (pk *Latency) ID() uint32 {
	return IDLatency
}

func (pk *Latency) Marshal(io protocol.IO) {
	io.Int64(&pk.Latency)
	io.Int64(&pk.Timestamp)
}
