package post_processing

import (
	"fmt"
	"testing"
)

func TestGetDevices(t *testing.T) {
	dirs, _ := GetDevices("/android/cpuprof-data")
	fmt.Println(dirs)
}
