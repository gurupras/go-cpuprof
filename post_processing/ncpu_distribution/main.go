package main

import (
	"os"

	"github.com/gurupras/cpuprof/post_processing"
)

func main() {
	post_processing.NcpuDistributionMain(os.Args)
}
