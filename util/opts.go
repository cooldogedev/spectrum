package util

// Opts defines the configuration options for Spectrum.
type Opts struct {
	// Addr is the address to listen on.
	Addr string `yaml:"addr"`
	// AutoLogin determines whether automatic login should be enabled.
	AutoLogin bool `yaml:"auto_login"`
	// ClientDecode is a list of client packet identifiers that need to be decoded by the proxy.
	ClientDecode []uint32
	// LatencyInterval is the interval at which the latency of the connection is updated in milliseconds.
	// Lower intervals provide more accurate latency but use more bandwidth.
	LatencyInterval int64 `yaml:"latency_interval"`
	// ShutdownMessage is the message displayed to clients when Spectrum shuts down.
	ShutdownMessage string `yaml:"shutdown_message"`
	// SyncProtocol determines the protocol version the proxy should use when communicating with servers.
	// When enabled, the proxy uses the client's protocol version (minecraft.Protocol) for reading and
	// writing packets. If disabled, the proxy defaults to using the latest protocol version (minecraft.DefaultProtocol).
	SyncProtocol bool `yaml:"sync_protocol"`
	// Token is the authentication token that Spectrum uses to authenticate with servers.
	Token string `yaml:"token"`
}

// ClientDecodeAsMap converts the ClientDecode slice to a map for faster lookups.
func (opts *Opts) ClientDecodeAsMap() map[uint32]struct{} {
	m := make(map[uint32]struct{}, len(opts.ClientDecode))
	for _, id := range opts.ClientDecode {
		m[id] = struct{}{}
	}
	return m
}

// DefaultOpts returns the default configuration options for Spectrum.
func DefaultOpts() *Opts {
	return &Opts{
		Addr:            ":19132",
		AutoLogin:       true,
		LatencyInterval: 3000,
		ShutdownMessage: "Spectrum closed.",
		SyncProtocol:    false,
	}
}
