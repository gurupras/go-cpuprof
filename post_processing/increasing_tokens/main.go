package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/gurupras/cpuprof"
	"github.com/gurupras/cpuprof/post_processing"
	"github.com/gurupras/cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons/gsync"
)

func increasingTokensBootConsumer(boot *post_processing.Boot, inChannel chan string, outChannel chan Empty) {
	var (
		logline      *cpuprof.Logline
		prev_logline *cpuprof.Logline
	)
	var line string
	var ok bool
	lines_processed := 0
	for {
		if line, ok = <-inChannel; !ok {
			break
		}
		lines_processed++
		if lines_processed%100000 == 0 {
			fmt.Println("increasingTokensConsumer: Processed:", lines_processed)
		}
		logline = cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}
		if prev_logline != nil {
			if logline.LogcatToken < prev_logline.LogcatToken {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("Tokens not increasing as expected: \n\t%v \n\t%v\n", prev_logline.Line, logline.Line))
				os.Exit(-1)
			}
		}
		prev_logline = logline
	}
	fmt.Println("Finished consuming bootid:", boot.BootId)
	empty := Empty{}
	outChannel <- empty
}

type Empty struct{}

func IncreasingTokensDistributionMain(args []string) {
	parser := post_processing.SetupParser()
	post_processing.ParseArgs(parser, args)

	deviceWg := new(sync.WaitGroup)
	deviceSem := gsync.NewSem(20)

	outChannel := make(chan Empty)

	processDevice := func(device string, boots []*post_processing.Boot) {
		defer deviceWg.Done()
		defer deviceSem.V()
		bootSem := gsync.NewSem(8)
		bootWg := new(sync.WaitGroup)

		processBoot := func(device string, boot *post_processing.Boot) {
			defer bootWg.Done()
			defer bootSem.V()
			lineChannel := make(chan string, 100000)

			f := func(line string) bool {
				return true
			}
			go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{f})

			increasingTokensBootConsumer(boot, lineChannel, outChannel)
		}

		for _, boot := range boots {
			bootWg.Add(1)
			bootSem.P()
			go processBoot(device, boot)
		}
		bootWg.Wait()
		fmt.Println("Finished processing Device:", device)
	}

	/* Main */
	device_files := post_processing.GetDeviceFiles(post_processing.Path, post_processing.Devices)
	for device, _ := range device_files {
		deviceWg.Add(1)
		fmt.Println("device:", device)
	}
	fn := func() {
		for device, boots := range device_files {
			deviceSem.P()
			go processDevice(device, boots)
		}
	}
	go fn()

	wg := new(sync.WaitGroup)
	outChannelConsumer := func() {
		defer wg.Done()
		var ok bool
		for {
			if _, ok = <-outChannel; !ok {
				break
			}
		}
	}
	wg.Add(1)
	go outChannelConsumer()
	deviceWg.Wait()
	close(outChannel)

	wg.Wait()
}

func main() {
	IncreasingTokensDistributionMain(os.Args)
}
