package post_processing

import (
	"fmt"
	"testing"

	"github.com/gurupras/go_cpuprof"
	"github.com/stretchr/testify/assert"
)

func TestBoot(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	boot := NewBoot("/android/cpuprof-data/", "0c037a6e55da4e024d9e64d97114c642695c5434", "453fea81-57cc-43e0-9693-91f63b0433b9")
	channel := make(chan string, 100000)
	go boot.AsyncRead(channel)

	var lastLogline *cpuprof.Logline = nil
	var line string
	var ok bool
	var lines int = 0

	for {
		if line, ok = <-channel; !ok {
			break
		}
		logline := cpuprof.ParseLogline(line)
		if lastLogline != nil {
			assert.True(logline.LogcatToken > lastLogline.LogcatToken, fmt.Sprintf("Loglines going backwards: \n%s\n%s\n", lastLogline.Line, logline.Line))
		}
		lastLogline = logline

		lines++
		if lines%200000 == 0 {
			fmt.Println("Lines:", lines)
		}
	}
}
