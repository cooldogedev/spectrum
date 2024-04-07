package spectrum

type Opts struct {
	// Addr is the address to listen on.
	Addr string `yaml:"addr"`
	// Token is the authentication token Spectrum clients have to use to authenticate with Spectrum.
	Token string `yaml:"token"`
	// LatencyInterval is the interval at which the latency of the connection is updated in milliseconds.
	// The lower the interval, the more accurate the latency will be, but the more bandwidth it will use.
	LatencyInterval int64 `yaml:"latency_interval"`
}

func DefaultOpts() *Opts {
	return &Opts{
		Addr:            ":19132",
		LatencyInterval: 3000,
	}
}
