package main

import (
	"flag"

	"github.com/alexbostock/part-ii-project/net"
)

func main() {
	opt := net.Options{
		flag.Uint("n", 5, "positive integer number of database nodes"),
		flag.Int64("seed", 0, "pseudorandom number generator seed"),
		flag.Float64("rate", 10, "average rate of transactions/second"),
		flag.Float64("latencymean", 20, "average network message latency in ms"),
		flag.Float64("latencyvar", 10, "variance of network message latency"),
		flag.Float64("nodefailchance", 0.01, "chance of a random node failing per client request"),
		flag.Uint("t", 100, "number of transactions"),
		flag.Float64("w", 0.1, "proportion of transactions which are writes"),
		flag.Bool("persistent", false, "use persistent data stores on disk rather than in-memory stores"),
		flag.Uint("vr", 3, "read quorum size"),
		flag.Uint("vw", 3, "write quorum size, must satisfy vw > n/2"),
		flag.Uint("numattempts", 1, "maximum number of attempts per transaction from the client"),
		flag.Bool("sloppy", false, "add background writes to provide eventually consistency in a sloppy quorum system"),
	}

	flag.Parse()

	net.Simulate(opt)
}
