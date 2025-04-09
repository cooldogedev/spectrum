package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

func init() {
	packet.RegisterPacketFromClient(IDConnectionRequest, func() packet.Packet { return &ConnectionRequest{} })
	packet.RegisterPacketFromClient(IDLatency, func() packet.Packet { return &Latency{} })

	packet.RegisterPacketFromServer(IDConnectionResponse, func() packet.Packet { return &ConnectionResponse{} })
	packet.RegisterPacketFromServer(IDFlush, func() packet.Packet { return &Flush{} })
	packet.RegisterPacketFromServer(IDLatency, func() packet.Packet { return &Latency{} })
	packet.RegisterPacketFromServer(IDTransfer, func() packet.Packet { return &Transfer{} })
	packet.RegisterPacketFromServer(IDUpdateCache, func() packet.Packet { return &UpdateCache{} })
}
