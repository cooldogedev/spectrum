package packet

var packets = map[uint32]func() Packet{}

func Register(id uint32, factory func() Packet) {
	packets[id] = factory
}

type Pool map[uint32]func() Packet

func NewPool() Pool {
	pool := Pool{}
	for id, factory := range packets {
		pool[id] = factory
	}
	return pool
}

func init() {
	Register(IDConnectionRequest, func() Packet { return &ConnectionRequest{} })
	Register(IDConnectionResponse, func() Packet { return &ConnectionResponse{} })
	Register(IDKick, func() Packet { return &Kick{} })
	Register(IDTransfer, func() Packet { return &Transfer{} })
}
