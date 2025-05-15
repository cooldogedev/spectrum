package api

import (
	"errors"
	"fmt"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
)

// Dial establishes a TCP connection to the specified API service address using the provided token.
// It returns a new Client instance if the connection and authentication are successful.
// Otherwise, it returns an error indicating the failure reason.
func Dial(addr, token string) (c *Client, err error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c = newClient(conn, packet.NewPool())
	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.WritePacket(&packet.ConnectionRequest{Token: token}); err != nil {
		return nil, err
	}

	connectionResponsePacket, err := c.ReadPacket()
	if err != nil {
		return nil, err
	}

	connectionResponse, ok := connectionResponsePacket.(*packet.ConnectionResponse)
	if !ok {
		return nil, fmt.Errorf("expected connection response, got %d", connectionResponse.ID())
	}

	if connectionResponse.Response == packet.ResponseFail {
		return nil, errors.New("connection failed")
	}

	if connectionResponse.Response == packet.ResponseUnauthorized {
		return nil, errors.New("connection unauthorized")
	}

	if connectionResponse.Response != packet.ResponseSuccess {
		return nil, fmt.Errorf("received an unknown response code %d", connectionResponse.ID())
	}
	return c, nil
}
