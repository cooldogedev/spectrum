package packet

// packets maps packet IDs to their respective factory functions.
var packets = map[uint32]func() Packet{}

// Register registers a packet factory function for a given ID.
func Register(id uint32, factory func() Packet) {
	packets[id] = factory
}

// Pool is a map holding packet factory functions indexed by their ID.
type Pool map[uint32]func() Packet

// NewPool creates a new Pool populated with registered packet factories.
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
