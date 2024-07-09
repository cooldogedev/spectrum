package util

// Opts defines the configuration options for Spectrum.
type Opts struct {
	// Addr is the address to listen on.
	Addr string `yaml:"addr"`
	// AutoLogin determines whether automatic login should be enabled.
	AutoLogin bool `yaml:"auto_login"`
	// LatencyInterval is the interval at which the latency of the connection is updated in milliseconds.
	// Lower intervals provide more accurate latency but use more bandwidth.
	LatencyInterval int64 `yaml:"latency_interval"`
	// Token is the authentication token that Spectrum uses to authenticate with servers.
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
