package session

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	errClosedNetworkConn = "closed network connection"
	errClosedStream      = "closed stream"
)

// handleServer continuously reads packets from the server and forwards them to the client.
func handleServer(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			server := s.Server()
			if !s.loggedIn {
				continue
			}

			pk, err := server.ReadPacket()
			if err != nil {
				if s.transferring.Load() || server != s.Server() {
					continue
				}

				if !s.closed.Load() {
					if isErrorLoggable(err) {
						s.logger.Error("failed to read packet from server", "err", err)
					}

					if err := s.fallback(); err == nil {
						continue
					}
				}
				return
			}

			switch pk := pk.(type) {
			case *packet2.Latency:
				s.serverLatency = pk.Latency
			case *packet2.Transfer:
				if err := s.Transfer(pk.Addr); err != nil {
					s.logger.Error("failed to transfer", "err", err)
				}
			case packet.Packet:
				ctx := NewContext()
				s.processor.ProcessServer(ctx, &pk)
				if ctx.Cancelled() {
					continue
				}

				if s.opts.SyncProtocol {
					for _, latest := range s.clientConn.Proto().ConvertToLatest(pk, s.clientConn) {
						s.tracker.handlePacket(latest)
					}
				} else {
					s.tracker.handlePacket(pk)
				}

				if err := s.clientConn.WritePacket(pk); err != nil {
					if isErrorLoggable(err) {
						s.logger.Error("failed to write packet to client", "err", err)
					}
					return
				}
			case []byte:
				ctx := NewContext()
				s.processor.ProcessServerEncoded(ctx, &pk)
				if ctx.Cancelled() {
					continue
				}

				if _, err := s.clientConn.Write(pk); err != nil {
					if isErrorLoggable(err) {
						s.logger.Error("failed to write raw packet to client", "err", err)
					}
					return
				}
			}
		}
	}
}

// handleClient continuously reads packets from the client and forwards them to the server.
func handleClient(s *Session) {
	defer s.Close()

	header := &packet.Header{}
	pool := s.clientConn.Proto().Packets(true)
	var deferredPackets [][]byte
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			payload, err := s.clientConn.ReadBytes()
			if err != nil {
				if isErrorLoggable(err) {
					s.logger.Error("failed to read packet from client", "err", err)
				}
				return
			}

			if !s.loggedIn {
				deferredPackets = append(deferredPackets, payload)
				continue
			}

			if len(deferredPackets) > 0 {
				for i, deferredPacket := range deferredPackets {
					if err := handleClientPacket(s, header, pool, deferredPacket); err != nil && isErrorLoggable(err) {
						s.logger.Error("failed to write deferred packet to server", "err", err)
					}
					deferredPackets[i] = nil
				}
				deferredPackets = deferredPackets[:0]
				deferredPackets = nil
			}

			if err := handleClientPacket(s, header, pool, payload); err != nil && isErrorLoggable(err) {
				s.logger.Error("failed to write packet to server", "err", err)
			}
		}
	}
}

// handleLatency periodically sends the client's current ping and timestamp to the server for latency reporting.
// Note: The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer func() {
		_ = s.Close()
		ticker.Stop()
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !s.loggedIn {
				continue
			}

			err := s.Server().WritePacket(&packet2.Latency{
				Latency:   s.clientConn.Latency().Milliseconds() * 2,
				Timestamp: time.Now().UnixMilli(),
			})
			if err != nil && !s.closed.Load() {
				s.logger.Error("failed to send latency packet", "err", err)
			}
		}
	}
}

// handleClientPacket processes and forwards the provided packet from the client to the server.
func handleClientPacket(s *Session, header *packet.Header, pool packet.Pool, payload []byte) (err error) {
	ctx := NewContext()
	if s.transferring.Load() {
		ctx.Cancel()
	}

	buf := bytes.NewBuffer(payload)
	if err := header.Read(buf); err != nil {
		return errors.New("failed to decode header")
	}

	if !slices.Contains(s.opts.ClientDecode, header.PacketID) {
		s.processor.ProcessClientEncoded(ctx, &payload)
		if !ctx.Cancelled() {
			return s.Server().Write(payload)
		}
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet %v: %v", header.PacketID, r)
		}
	}()

	factory, ok := pool[header.PacketID]
	if !ok {
		return fmt.Errorf("unknown packet %d", header.PacketID)
	}

	pk := factory()
	pk.Marshal(s.clientConn.Proto().NewReader(buf, s.shieldID, true))
	s.processor.ProcessClient(ctx, &pk)
	if !ctx.Cancelled() {
		if s.opts.SyncProtocol {
			return s.Server().WritePacket(pk)
		}

		for _, latest := range s.clientConn.Proto().ConvertToLatest(pk, s.clientConn) {
			if err := s.Server().WritePacket(latest); err != nil {
				return err
			}
		}
	}
	return
}

func isErrorLoggable(err error) bool {
	return !errors.Is(err, net.ErrClosed) && !strings.Contains(err.Error(), errClosedStream) && !strings.Contains(err.Error(), errClosedNetworkConn)
}
