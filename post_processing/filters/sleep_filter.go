package filters

import (
	"fmt"
	"os"
	"strings"

	"github.com/gurupras/cpuprof"
)

type SuspendState int

const (
	SUSPEND_STATE_UNKNOWN   SuspendState = 1 << iota
	SUSPEND_STATE_SUSPENDED SuspendState = 1 << iota
	SUSPEND_STATE_AWAKE     SuspendState = 1 << iota
)

type SleepFilter struct {
	*Filter
	Exclusive            bool
	CurrentState         SuspendState
	FilterState          SuspendState
	lastSuspendEntry     *cpuprof.PowerManagementPrintk
	SuspendEntryCallback func(pmp *cpuprof.PowerManagementPrintk)
	SuspendExitCallback  func(pmp *cpuprof.PowerManagementPrintk)
	FilterFunc           LineFilter
	Log                  bool
}

func NewSleepFilter(filter *Filter) (sleepFilter *SleepFilter) {
	sleepFilter = new(SleepFilter)
	sleepFilter.Filter = filter
	sleepFilter.CurrentState = SUSPEND_STATE_UNKNOWN
	sleepFilter.FilterState = SUSPEND_STATE_AWAKE
	sleepFilter.Exclusive = false
	sleepFilter.SuspendEntryCallback = nil
	sleepFilter.SuspendExitCallback = nil
	sleepFilter.Log = false

	filterFunc := func(logline *cpuprof.Logline) bool {
		result := false

		log := func(message string) {
			if sleepFilter.Log {
				fmt.Fprintln(os.Stderr, message)
			}
		}

		if strings.Contains(logline.Line, "KernelPrintk") &&
			(strings.Contains(logline.Line, "PM: suspend entry") ||
				strings.Contains(logline.Line, "PM: suspend exit")) {

			result = true

			pmp := cpuprof.ParsePowerManagementPrintk(logline)

			if pmp == nil {
				panic(fmt.Sprintf("Failed to parse: %v", logline.Line))
			}

			switch pmp.State {
			case cpuprof.PM_SUSPEND_ENTRY:
				if sleepFilter.CurrentState == SUSPEND_STATE_SUSPENDED {
					log("Suspend when suspended?")
				}
				sleepFilter.CurrentState = SUSPEND_STATE_SUSPENDED
				if sleepFilter.SuspendEntryCallback != nil {
					sleepFilter.SuspendEntryCallback(pmp)
				}
				sleepFilter.lastSuspendEntry = pmp
			case cpuprof.PM_SUSPEND_EXIT:
				if sleepFilter.CurrentState != SUSPEND_STATE_SUSPENDED {
					log("Suspend exit when not suspended??")
				} else {
					if sleepFilter.lastSuspendEntry == nil {
						log("Suspend exit when lastSuspendEntry is nil??")
						os.Exit(-1)
					}
				}
				sleepFilter.CurrentState = SUSPEND_STATE_AWAKE
				if sleepFilter.SuspendExitCallback != nil {
					sleepFilter.SuspendExitCallback(pmp)
				}
			}
		}
		if sleepFilter.Exclusive {
			return result
		}

		if sleepFilter.CurrentState&sleepFilter.FilterState != 0 {
			return true
		}
		return false
	}
	filter.AddFilter(filterFunc)
	return sleepFilter
}
