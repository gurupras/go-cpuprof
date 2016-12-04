package post_processing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/gurupras/cpuprof"
	"github.com/gurupras/cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons/gsync"
)

func ncpuBootConsumer(boot *Boot, inChannel chan string, outChannel chan map[int]float64) float64 {
	var (
		logline           *cpuprof.Logline
		firstLogline      *cpuprof.Logline
		lastLogline       *cpuprof.Logline
		stateStartLogline *cpuprof.Logline
		stateEndLogline   *cpuprof.Logline
	)
	psm := NewPhoneStateMachine(4)

	ncpuMap := make(map[int]float64)

	logState := func() {
		duration := stateEndLogline.TraceTime - stateStartLogline.TraceTime

		ncpus := psm.NumOnlineCpus
		if _, ok := ncpuMap[ncpus]; !ok {
			ncpuMap[ncpus] = 0.0
		}
		ncpuMap[ncpus] += duration

		// The current logline becomes the start for the next state
		stateStartLogline = logline
		// Reset state end
		stateEndLogline = nil
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
			fmt.Println("ncpuBootConsumer: Processed:", lines_processed)
		}
		logline = cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}

		var doesTraceAffectState bool = false
		trace := cpuprof.ParseTraceFromLoglinePayload(logline)
		if trace == nil {
			fmt.Fprintln(os.Stderr, "Trace is nil:", line)
			continue
		}

		switch trace.Tag() {
		case "phonelab_num_online_cpus":
			stateEndLogline = logline
			pnoc := trace.(*cpuprof.PhonelabNumOnlineCpus)
			ncpu := pnoc.NumOnlineCpus
			// Before we can set state, we need to check if we have fullstate
			if psm.NumOnlineCpus != -1 {
				// Log state
				logState()
			}
			psm.NumOnlineCpus = ncpu
			doesTraceAffectState = true
		}
		if doesTraceAffectState == true {
			if stateStartLogline == nil {
				// We have no previous start
				stateStartLogline = logline
			}
			/*
				if stateStartLogline != nil && stateEndLogline != nil && stateStartLogline != stateEndLogline {
					// This should not have happened. It should've been logged earlier
					fmt.Fprintln(os.Stderr, "State start and end available, but state wasn't logged")
					os.Exit(-1)
				}
			*/
		}
		if firstLogline == nil {
			firstLogline = logline
		}
		lastLogline = logline
	}
	fmt.Println("Finished consuming bootid:", boot.BootId)
	if lastLogline != nil && firstLogline != nil {
		bootDuration := lastLogline.TraceTime - firstLogline.TraceTime
		for k, v := range ncpuMap {
			ncpuMap[k] = v / bootDuration
		}
		outChannel <- ncpuMap
		return bootDuration
	} else {
		fmt.Fprintln(os.Stderr, "Either first/last logline was nil..no map")
		return 0.0
	}
}

func NcpuDistributionMain(args []string) {
	parser := setup_parser()
	ParseArgs(parser, args)

	deviceWg := new(sync.WaitGroup)
	deviceSem := gsync.NewSem(20)

	outChannel := make(chan map[int]float64, 100000)

	processDevice := func(device string, boots []*Boot) {
		defer deviceWg.Done()
		defer deviceSem.V()
		bootSem := gsync.NewSem(8)
		bootWg := new(sync.WaitGroup)

		processBoot := func(device string, boot *Boot) {
			defer bootWg.Done()
			defer bootSem.V()
			lineChannel := make(chan string, 100000)

			f := func(line string) bool {
				return strings.Contains(line, "Kernel-Trace") && strings.Contains(line, "phonelab_num_online_cpus")
			}
			go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{f})

			ncpuBootConsumer(boot, lineChannel, outChannel)
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
	device_files := GetDeviceFiles(Path, Devices)
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

	ncpuUsageMap := make(map[string][]float64)

	wg := new(sync.WaitGroup)
	outChannelConsumer := func() {
		defer wg.Done()
		var ncpuMap map[int]float64
		var ok bool
		for {
			if ncpuMap, ok = <-outChannel; !ok {
				break
			}
			for k, v := range ncpuMap {
				key := fmt.Sprintf("%v", k)
				if _, ok := ncpuUsageMap[key]; !ok {
					ncpuUsageMap[key] = make([]float64, 0)
				}
				ncpuUsageMap[key] = append(ncpuUsageMap[key], v)
			}
		}
	}
	wg.Add(1)
	go outChannelConsumer()
	deviceWg.Wait()
	close(outChannel)

	wg.Wait()
	if b, err := json.MarshalIndent(ncpuUsageMap, "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		ioutil.WriteFile("ncpu-output.json", b, 0664)
	}
}
