package post_processing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/gurupras/cpuprof"
	"github.com/gurupras/cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons/gsync"
)

type CpuState int

const (
	CPU_STATE_UNKNOWN CpuState = iota
	CPU_STATE_ONLINE           = iota
	CPU_STATE_OFFLINE          = iota
)

var CpuStates = [...]string{
	"CPU_STATE_UNKNOWN",
	"CPU_STATE_ONLINE",
	"CPU_STATE_OFFLINE",
}

type CpuStateMachine struct {
	Cpu       int
	State     CpuState
	Frequency int
}

func (csm *CpuStateMachine) ChangeState(state string) {
	switch state {
	case "online":
		csm.State = CPU_STATE_ONLINE
	case "offline":
		csm.State = CPU_STATE_OFFLINE
		csm.Frequency = -1
	}
}

func (csm *CpuStateMachine) ChangeFrequency(frequency int) (err error) {
	if csm.State == CPU_STATE_UNKNOWN {
		csm.State = CPU_STATE_ONLINE
	}
	if csm.State != CPU_STATE_ONLINE {
		//errMsg := fmt.Sprintf("Trying to set frequency to a CPU that is %s?", CpuStates[csm.State])
		//fmt.Fprintln(os.Stderr, errMsg)
		//err = errors.New(errMsg)
	}
	csm.Frequency = frequency
	return
}

type PhoneStateMachine struct {
	Ncpus           int
	NumOnlineCpus   int
	CpuStateMachine []*CpuStateMachine
}

func NewPhoneStateMachine(ncpus int) *PhoneStateMachine {
	psm := new(PhoneStateMachine)
	psm.Ncpus = ncpus
	psm.NumOnlineCpus = -1
	psm.CpuStateMachine = make([]*CpuStateMachine, 4)
	for idx := 0; idx < 4; idx++ {
		sm := new(CpuStateMachine)
		sm.Cpu = idx
		sm.State = CPU_STATE_UNKNOWN
		sm.Frequency = -1
		psm.CpuStateMachine[idx] = sm
	}
	return psm
}

func (psm *PhoneStateMachine) FullState() bool {
	fullState := true
	for idx := 0; idx < psm.Ncpus; idx++ {
		csm := psm.CpuStateMachine[idx]
		if csm.State == CPU_STATE_UNKNOWN {
			fullState = false
			break
		} else if csm.State == CPU_STATE_ONLINE {
			if csm.Frequency == -1 {
				fullState = false
				break
			}
		}
	}
	return fullState
}

func bootConsumer(boot *Boot, inChannel chan string, outChannel chan map[string]float64) float64 {
	var (
		logline           *cpuprof.Logline
		firstLogline      *cpuprof.Logline
		lastLogline       *cpuprof.Logline
		stateStartLogline *cpuprof.Logline
		stateEndLogline   *cpuprof.Logline
	)
	psm := NewPhoneStateMachine(4)

	frequencyMap := make(map[string]float64)

	logState := func() {
		duration := stateEndLogline.TraceTime - stateStartLogline.TraceTime

		frequencies := make([]string, 0)
		for _, csm := range psm.CpuStateMachine {
			if csm.State == CPU_STATE_ONLINE {
				frequencies = append(frequencies, fmt.Sprintf("%v", csm.Frequency))
			}
		}
		sort.Strings(frequencies)
		key := strings.Join(frequencies, "-")
		if _, ok := frequencyMap[key]; !ok {
			frequencyMap[key] = 0.0
		}
		frequencyMap[key] += duration

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
			fmt.Println("bootConsumer: Processed:", lines_processed)
		}
		logline = cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}

		var doesTraceAffectState bool = false
		trace := cpuprof.ParseTraceFromLoglinePayload(logline)
		if trace == nil {
			continue
		}

		switch trace.Tag() {
		case "sched_cpu_hotplug":
			stateEndLogline = logline
			sch := trace.(*cpuprof.SchedCpuHotplug)
			if sch.Error != 0 {
				continue
			}
			cpu := sch.Cpu
			state := sch.State
			csm := psm.CpuStateMachine[cpu]
			// Before we can set state, we need to check if we have fullstate
			if psm.FullState() {
				// Log state
				logState()
			}
			csm.ChangeState(state)
			doesTraceAffectState = true
		case "cpu_frequency":
			stateEndLogline = logline
			cf := trace.(*cpuprof.CpuFrequency)
			cpu := cf.CpuId
			// Before we can set frequency, we need to check if we have fullstate
			if psm.FullState() {
				logState()
			}
			if err := psm.CpuStateMachine[cpu].ChangeFrequency(cf.State); err != nil {
				fmt.Fprintln(os.Stderr, err)
				fmt.Fprintln(os.Stderr, line)
				os.Exit(-1)
			}
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
		for k, v := range frequencyMap {
			frequencyMap[k] = v / bootDuration
		}
		outChannel <- frequencyMap
		return bootDuration
	} else {
		return 0.0
	}
}

func FrequencyDistributionMain(args []string) {
	parser := setup_parser()
	ParseArgs(parser, args)

	deviceWg := new(sync.WaitGroup)
	deviceSem := gsync.NewSem(20)

	outChannel := make(chan map[string]float64, 100000)

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
				return strings.Contains(line, "Kernel-Trace") && (strings.Contains(line, "cpu_frequency") || strings.Contains(line, "sched_cpu_hotplug"))
			}
			go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{f})

			bootConsumer(boot, lineChannel, outChannel)
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

	frequencyUsageMap := make(map[string][]float64)

	wg := new(sync.WaitGroup)
	outChannelConsumer := func() {
		defer wg.Done()
		frequencyMap := make(map[string]float64)
		var ok bool
		for {
			if frequencyMap, ok = <-outChannel; !ok {
				break
			}
			for k, v := range frequencyMap {
				if _, ok := frequencyUsageMap[k]; !ok {
					frequencyUsageMap[k] = make([]float64, 0)
				}
				frequencyUsageMap[k] = append(frequencyUsageMap[k], v)
			}
		}
	}
	wg.Add(1)
	go outChannelConsumer()
	deviceWg.Wait()
	close(outChannel)

	wg.Wait()
	b, _ := json.MarshalIndent(frequencyUsageMap, "", "  ")
	ioutil.WriteFile("output.json", b, 0664)
}
