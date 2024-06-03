package util

// Opts defines the configuration options for Spectrum.
type Opts struct {
	// Addr is the address to listen on.
	Addr string `yaml:"addr"`
	// AutoLogin determines whether automatic login should be enabled.
	AutoLogin bool `yaml:"auto_login"`
	// LatencyInterval is the interval at which the latency of the connection is updated in milliseconds.
	// The lower the interval, the more accurate the latency will be, but the more bandwidth it will use.
	LatencyInterval int64 `yaml:"latency_interval"`
	// Token is the authentication token that Spectrum uses to authenticate with servers.
	// When making requests to servers, Spectrum sends this token to the server for validation.
	Token string `yaml:"token"`
}

// DefaultOpts returns the default configuration options for Spectrum.
func DefaultOpts() *Opts {
	return &Opts{
		Addr:            ":19132",
		AutoLogin:       true,
		LatencyInterval: 3000,
	}
}
