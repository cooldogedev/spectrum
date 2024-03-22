package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

func init() {
	packet.RegisterPacketFromClient(IDConnect, func() packet.Packet { return &Connect{} })
	packet.RegisterPacketFromClient(IDLatency, func() packet.Packet { return &Latency{} })

	packet.RegisterPacketFromServer(IDLatency, func() packet.Packet { return &Latency{} })
	packet.RegisterPacketFromServer(IDTransfer, func() packet.Packet { return &Transfer{} })
}
