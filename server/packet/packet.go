package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

func init() {
	packet.RegisterPacketFromClient(IDConnectionRequest, func() packet.Packet { return &ConnectionRequest{} })
	packet.RegisterPacketFromClient(IDLatency, func() packet.Packet { return &Latency{} })

	packet.RegisterPacketFromServer(IDConnectionResponse, func() packet.Packet { return &ConnectionResponse{} })
	packet.RegisterPacketFromServer(IDLatency, func() packet.Packet { return &Latency{} })
	packet.RegisterPacketFromServer(IDTransfer, func() packet.Packet { return &Transfer{} })
}
