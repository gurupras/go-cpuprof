package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/gurupras/cpuprof"
	"github.com/gurupras/cpuprof/post_processing"
	"github.com/gurupras/cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons/gsync"
)

func tempDistributionBootConsumer(boot *post_processing.Boot, inChannel chan string, outChannel chan map[int]int64) {
	var (
		logline *cpuprof.Logline
	)
	thermalMap := make(map[int]int64)

	logState := func(temp int) {
		if _, ok := thermalMap[temp]; !ok {
			thermalMap[temp] = 0
		}
		thermalMap[temp]++
	}

	var line string
	var ok bool
	lines_processed := 0
	for {
		if line, ok = <-inChannel; !ok {
			break
		}
		lines_processed++
		if lines_processed%100000 == 0 {
			fmt.Println("tempDistributionBootConsumer: Processed:", lines_processed)
		}
		logline = cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}

		trace := cpuprof.ParseTraceFromLoglinePayload(logline)
		if trace == nil {
			fmt.Fprintln(os.Stderr, "Trace is nil:", line)
			continue
		}

		switch trace.Tag() {
		case "thermal_temp":
			tt := trace.(*cpuprof.ThermalTemp)
			temp := tt.Temp
			// Log state
			logState(temp)
		}
	}
	fmt.Println("Finished consuming bootid:", boot.BootId)
	outChannel <- thermalMap
}

func TempDistributionMain(args []string) {
	parser := post_processing.SetupParser()
	post_processing.ParseArgs(parser, args)

	deviceWg := new(sync.WaitGroup)
	deviceSem := gsync.NewSem(20)

	outChannel := make(chan map[int]int64, 100000)

	processDevice := func(device string, boots []*post_processing.Boot) {
		defer deviceWg.Done()
		defer deviceSem.V()
		bootSem := gsync.NewSem(8)
		bootWg := new(sync.WaitGroup)

		processBoot := func(device string, boot *post_processing.Boot) {
			defer bootWg.Done()
			defer bootSem.V()
			lineChannel := make(chan string, 100000)

			// We only count temperatures during active use (foreground!=0)
			var pid int = 0
			f := func(line string) bool {
				inActiveUse := post_processing.BaseForegroundFilter(line, &pid)
				return inActiveUse && (strings.Contains(line, "Kernel-Trace") && strings.Contains(line, "thermal_temp"))
			}
			go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{f})

			tempDistributionBootConsumer(boot, lineChannel, outChannel)
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

	thermalUsageMap := make(map[string]int64)

	wg := new(sync.WaitGroup)
	outChannelConsumer := func() {
		defer wg.Done()
		var thermalMap map[int]int64
		var ok bool
		for {
			if thermalMap, ok = <-outChannel; !ok {
				break
			}
			for k, v := range thermalMap {
				key := fmt.Sprintf("%v", k)
				if _, ok := thermalUsageMap[key]; !ok {
					thermalUsageMap[key] = 0
				}
				thermalUsageMap[key] += v
			}
		}
	}
	wg.Add(1)
	go outChannelConsumer()
	deviceWg.Wait()
	close(outChannel)

	wg.Wait()
	if b, err := json.MarshalIndent(thermalUsageMap, "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		ioutil.WriteFile("temp_distribution.json", b, 0664)
	}
}

func main() {
	TempDistributionMain(os.Args)
}
