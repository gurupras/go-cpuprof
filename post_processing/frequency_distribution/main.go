package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gurupras/go_cpuprof"
	"github.com/gurupras/go_cpuprof/post_processing"
	"github.com/gurupras/go_cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons/gsync"
)

func processBoot(deviceId string, boot *post_processing.Boot, outChannel chan map[string]float64, wg *sync.WaitGroup, sem *gsync.Semaphore) {
	defer sem.V()
	defer wg.Done()
	lineChannel := make(chan string, 100000)

	f := func(line string) bool {
		return strings.Contains(line, "Kernel-Trace") && (strings.Contains(line, "cpu_frequency:") || strings.Contains(line, "sched_cpu_hotplug:"))
	}
	go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{f})

	bootConsumer(boot, lineChannel, outChannel)
}

func bootConsumer(boot *post_processing.Boot, inChannel chan string, outChannel chan map[string]float64) {

	frequencyMap := make(map[string]float64)

	filter := filters.New()
	cpuTracker := filters.NewCpuTracker(filter)

	callback := func(trace cpuprof.TraceInterface, cpu int) {
		account := func(trace *cpuprof.Trace) {
			if cpuTracker.CurrentState[cpu].Frequency != filters.FREQUENCY_STATE_UNKNOWN {
				// There was a legitimate previous frequency. Account time spent in it
				start := cpuTracker.CurrentState[cpu].FrequencyLogline.TraceTime
				end := trace.Logline.TraceTime

				freqStr := fmt.Sprintf("%v", cpuTracker.CurrentState[cpu].Frequency)
				if _, ok := frequencyMap[freqStr]; !ok {
					frequencyMap[freqStr] = 0.0
				}
				frequencyMap[freqStr] += (end - start)
			}
		}

		switch trace.Tag() {
		case "cpu_frequency":
			cf := trace.(*cpuprof.CpuFrequency)
			cpu := cf.CpuId
			// Track time spent in 'current' state
			if _, ok := cpuTracker.CurrentState[cpu]; !ok {
				return
			}
			account(cf.Trace)
		case "sched_cpu_hotplug":
			sch := trace.(*cpuprof.SchedCpuHotplug)
			if sch.State != "offline" {
				break
			}
			// This CPU just went offline. Account time
			account(sch.Trace)
		}
	}
	cpuTracker.Callback = callback
	var line string
	var ok bool
	lines_processed := 0
	for {
		if line, ok = <-inChannel; !ok {
			break
		}
		//fmt.Println(line)
		lines_processed++
		if lines_processed%100000 == 0 {
			//fmt.Println("bootConsumer: Processed:", lines_processed)
		}
		filter.Apply(line)
	}
	outChannel <- frequencyMap
}

func Main(args []string) {
	parser := post_processing.SetupParser()
	post_processing.ParseArgs(parser, args)

	deviceWg := new(sync.WaitGroup)
	deviceSem := gsync.NewSem(20)

	outChannel := make(chan map[string]float64, 100000)

	device_files := post_processing.GetDeviceFiles(post_processing.Path, post_processing.Devices)

	var doneDevices int32 = 0
	processDevice := func(device string, boots []*post_processing.Boot) {
		defer deviceWg.Done()
		defer deviceSem.V()
		bootSem := gsync.NewSem(8)
		bootWg := new(sync.WaitGroup)

		doneBoots := int32(0)
		bootFn := func(boot *post_processing.Boot) {
			processBoot(device, boot, outChannel, bootWg, bootSem)
			atomic.AddInt32(&doneBoots, 1)
			fmt.Println(fmt.Sprintf("%v -> %v Done! (%d/%d)", device, boot.BootId, doneBoots, len(boots)))
		}
		for _, boot := range boots {
			bootWg.Add(1)
			bootSem.P()
			go bootFn(boot)
		}
		bootWg.Wait()
		atomic.AddInt32(&doneDevices, 1)
		fmt.Println(fmt.Sprintf("Finished processing Device: %v (%v/%v)", device, doneDevices, len(device_files)))
	}

	/* Main */
	for device, boots := range device_files {
		deviceSem.P()
		deviceWg.Add(1)
		go processDevice(device, boots)
	}

	frequencyUsageMap := make(map[string]float64)

	wg := new(sync.WaitGroup)
	outChannelConsumer := func() {
		defer wg.Done()
		var frequencyMap map[string]float64
		var ok bool
		for {
			if frequencyMap, ok = <-outChannel; !ok {
				break
			}
			for k, _ := range frequencyMap {
				if _, ok := frequencyUsageMap[k]; !ok {
					frequencyUsageMap[k] = 0.0
				}
				frequencyUsageMap[k] += frequencyMap[k]
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

func main() {
	Main(os.Args)
}
