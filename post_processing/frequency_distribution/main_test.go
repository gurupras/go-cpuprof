package main

import (
	"testing"

	"github.com/google/shlex"
	"github.com/gurupras/cpuprof/post_processing"
)

func TestFrequencyDistribution(t *testing.T) {
	cmdline, _ := shlex.Split("./frequency_distribution /android/cpuprof-data -d 6f3cdb988ff27b78ca2df7e32268c74fc54925dc")
	post_processing.FrequencyDistributionMain(cmdline)
}
