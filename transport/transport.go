package transport

import "io"

type Transport interface {
	Dial(addr string) (io.ReadWriteCloser, error)
}
