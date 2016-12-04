package filters

import (
	"fmt"
	"os"
	"strings"

	"github.com/gurupras/cpuprof"
)

type CpuState int

const (
	FREQUENCY_STATE_UNKNOWN int = 0
)

const (
	CPU_STATE_UNKNOWN CpuState = -1
	CPU_OFFLINE       CpuState = 0
	CPU_ONLINE        CpuState = 1
)

type CpuTrackerData struct {
	Cpu              int
	CpuState         CpuState
	Frequency        int
	FrequencyLogline *cpuprof.Logline
	CpuStateLogline  *cpuprof.Logline
}

type CpuTracker struct {
	*Filter
	Exclusive    bool
	CurrentState map[int]*CpuTrackerData
	Callback     func(trace cpuprof.TraceInterface, cpu int)
	FilterFunc   LoglineFilter
}

func NewCpuTracker(filter *Filter) (cpuTracker *CpuTracker) {
	cpuTracker = new(CpuTracker)
	cpuTracker.Filter = filter
	cpuTracker.CurrentState = make(map[int]*CpuTrackerData)
	cpuTracker.Callback = nil

	filterFunc := func(logline *cpuprof.Logline) bool {
		if strings.Contains(logline.Line, "cpu_frequency:") || strings.Contains(logline.Line, "sched_cpu_hotplug:") {
			trace := cpuprof.ParseTraceFromLoglinePayload(logline)
			if trace == nil {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("Trace is nil: %v", logline.Line))
				return true
			}

			var cpu int
			var ctd *CpuTrackerData
			switch trace.Tag() {
			case "cpu_frequency":
				cf := trace.(*cpuprof.CpuFrequency)
				cpu = cf.CpuId
				if _, ok := cpuTracker.CurrentState[cpu]; !ok {
					cpuTracker.CurrentState[cpu] = new(CpuTrackerData)
					cpuTracker.CurrentState[cpu].Cpu = cpu
					cpuTracker.CurrentState[cpu].CpuState = CPU_STATE_UNKNOWN
					cpuTracker.CurrentState[cpu].Frequency = FREQUENCY_STATE_UNKNOWN
				}

				cf.Trace.Logline = logline
				if cpuTracker.Callback != nil {
					cpuTracker.Callback(trace, cpu)
				}

				ctd = cpuTracker.CurrentState[cpu]
				if ctd.CpuState != CPU_ONLINE {
					// This CPU is clearly up
					ctd.CpuStateLogline = logline
					ctd.CpuState = CPU_ONLINE
				}

				ctd.Frequency = cf.State
				ctd.FrequencyLogline = logline
			case "sched_cpu_hotplug":
				sch := trace.(*cpuprof.SchedCpuHotplug)
				cpu = sch.Cpu
				if _, ok := cpuTracker.CurrentState[cpu]; !ok {
					cpuTracker.CurrentState[cpu] = new(CpuTrackerData)
					cpuTracker.CurrentState[cpu].Cpu = cpu
					cpuTracker.CurrentState[cpu].CpuState = CPU_STATE_UNKNOWN
					cpuTracker.CurrentState[cpu].Frequency = FREQUENCY_STATE_UNKNOWN
				}
				sch.Trace.Logline = logline
				if cpuTracker.Callback != nil {
					cpuTracker.Callback(trace, cpu)
				}
				ctd = cpuTracker.CurrentState[cpu]
				if strings.Compare(sch.State, "offline") == 0 && sch.Error == 0 {
					// This core just went offline
					ctd.CpuState = CPU_OFFLINE
					ctd.CpuStateLogline = logline
				} else if strings.Compare(sch.State, "online") == 0 && sch.Error == 0 {
					ctd.CpuState = CPU_ONLINE
					ctd.CpuStateLogline = logline
				}
			}
		}
		return true
	}
	cpuTracker.FilterFunc = filterFunc
	filter.AddFilter(filterFunc)
	return cpuTracker
}
