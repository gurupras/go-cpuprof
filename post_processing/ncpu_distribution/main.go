package main

import (
	"os"

	"github.com/gurupras/go_cpuprof/post_processing"
)

func main() {
	post_processing.NcpuDistributionMain(os.Args)
}
