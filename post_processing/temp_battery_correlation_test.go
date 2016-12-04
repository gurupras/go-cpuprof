package post_processing

import (
	"fmt"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
)

func TestCheckIfDone(t *testing.T) {
	result := checkIfDone("/android/temp_battery_analysis/test/0000000000000000000000000000000000000000/analysis/temp_battery")
	assert.Equal(t, true, result, "Failed checkIfDone")

	result = checkIfDone("/android/cpuprof-data/67a183f2a61bf5cbb02a3641db8386e3686bd324/analysis/temp_battery")
	assert.Equal(t, false, result, "Failed checkIfDone")
}

func TestTBC(t *testing.T) {
	fmt.Println("TestTBC")
	args, _ := shlex.Split("tbc /android/temp_battery_analysis/test/ --device 8d0376587d6091bed78e081b614ca485fb098c23")
	_ = "breakpoint"
	TBCMain(args)
}

func TestTBCSerializer(t *testing.T) {
	fmt.Println("TestTBC")
	args, _ := shlex.Split("tbc /android/temp_battery_analysis/test/ --device 0000000000000000000000000000000000000000 --reload")
	_ = "breakpoint"
	TBCMain(args)
}
