package main

import (
	"log"
	"os"
	"strconv"

	"github.com/alexbostock/part-ii-project/simnet"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: part-ii-project <number of db nodes>")
	}

	n, err := strconv.ParseUint(os.Args[1], 10, 32)

	if err != nil {
		log.Fatal("Usage: part-ii-project <number of db nodes>")
	}

	simnet.Simulate(uint(n))
}
