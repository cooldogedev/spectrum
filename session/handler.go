package session

import (
	"strings"
	"time"

	"github.com/cooldogedev/spectrum/server/packet"
	packet2 "github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func handleIncoming(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.closed:
			return
		default:
			if s.transferring.Load() {
				continue
			}

			server := s.Server()
			pk, err := server.ReadPacket()
			if err != nil {
				if server != s.Server() {
					continue
				}

				s.logger.Errorf("Failed to read packet from server: %v", err)
				return
			}

			switch pk := pk.(type) {
			case *packet.Latency:
				s.latency = pk.Latency
			case *packet.Transfer:
				if err := s.Transfer(pk.Addr); err != nil {
					s.logger.Errorf("Failed to transfer: %v", err)
				}
			case packet2.Packet:
				if s.processor != nil && !s.processor.ProcessServer(pk) {
					continue
				}

				s.tracker.handlePacket(pk)
				if err := s.clientConn.WritePacket(pk); err != nil {
					s.logger.Errorf("Failed to write packet to client: %v", err)
					return
				}
			case []byte:
				if _, err := s.clientConn.Write(pk); err != nil {
					s.logger.Errorf("Failed to write raw packet to client: %v", err)
					return
				}
			}
		}
	}
}

func handleOutgoing(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.closed:
			return
		default:
			if s.transferring.Load() {
				continue
			}

			pk, err := s.clientConn.ReadPacket()
			if err != nil {
				if !strings.Contains(err.Error(), "closed network connection") {
					s.logger.Errorf("Failed to read packet from client: %v", err)
				}
				return
			}

			if violation, ok := pk.(*packet2.PacketViolationWarning); ok {
				s.logger.Errorf("Received packet violation warning: PacketID=%v, Context=%v, Severity=%v, Type=%v", violation.PacketID, violation.ViolationContext, violation.Severity, violation.Type)
			}

			if s.processor != nil && !s.processor.ProcessClient(pk) {
				continue
			}

			if err := s.Server().WritePacket(pk); err != nil {
				s.logger.Errorf("Failed to write packet to server: %v", err)
				return
			}
		}
	}
}

func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer ticker.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-ticker.C:
			if s.transferring.Load() {
				continue
			}

			if err := s.Server().WritePacket(&packet.Latency{Latency: s.clientConn.Latency().Milliseconds(), Timestamp: time.Now().UnixMilli()}); err != nil {
				s.logger.Errorf("Failed to send latency packet: %v", err)
				return
			}
		}
	}
}
