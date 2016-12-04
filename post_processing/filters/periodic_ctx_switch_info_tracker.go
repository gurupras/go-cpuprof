package filters

import (
	"fmt"
	"os"

	"github.com/gurupras/cpuprof"
)

type PeriodicCtxSwitchInfoTracker struct {
	*Filter
	CtxSwitchInfo map[int]*cpuprof.PeriodicCtxSwitchInfo
	Callback      func(ctxSwitchInfo *cpuprof.PeriodicCtxSwitchInfo)
	FilterFunc    LoglineFilter
}

func NewPeriodicCtxSwitchInfoTracker(filter *Filter) (pcsiTracker *PeriodicCtxSwitchInfoTracker) {
	pcsiTracker = new(PeriodicCtxSwitchInfoTracker)
	pcsiTracker.Filter = filter
	pcsiTracker.CtxSwitchInfo = make(map[int]*cpuprof.PeriodicCtxSwitchInfo)
	pcsiTracker.Callback = nil

	filterFunc := func(logline *cpuprof.Logline) bool {
		trace := cpuprof.ParseTraceFromLoglinePayload(logline)
		if trace == nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Trace is nil: %v", logline.Line))
			return true
		}

		var cpu int
		switch trace.Tag() {
		case "phonelab_periodic_ctx_switch_marker":
			ppcsm := trace.(*cpuprof.PhonelabPeriodicCtxSwitchMarker)
			cpu = ppcsm.Cpu
			switch ppcsm.State {
			case cpuprof.PPCSMBegin:
				if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; v == nil || !ok {
					pcsiTracker.CtxSwitchInfo[cpu] = new(cpuprof.PeriodicCtxSwitchInfo)
					pcsiTracker.CtxSwitchInfo[cpu].Info = make([]*cpuprof.PhonelabPeriodicCtxSwitchInfo, 0)
				} else if v != nil {
					//fmt.Fprintln(os.Stderr, "Start logline when already started")
					//fmt.Fprintln(os.Stderr, logline.Line)
					//os.Exit(-1)
					return true
				}
				pcsiTracker.CtxSwitchInfo[cpu].Start = ppcsm
			case cpuprof.PPCSMEnd:
				if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; !ok || v == nil {
					// End marker without begin..ignore
					return true
				}
				pcsi := pcsiTracker.CtxSwitchInfo[cpu]
				pcsi.End = ppcsm
				if pcsiTracker.Callback != nil {
					pcsiTracker.Callback(pcsi)
				}
				pcsiTracker.CtxSwitchInfo[cpu] = nil
			}
		case "phonelab_periodic_ctx_switch_info":
			ppcsi := trace.(*cpuprof.PhonelabPeriodicCtxSwitchInfo)
			cpu = ppcsi.Cpu
			if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; v == nil || !ok {
				// Info line without begin marker.. ignore
				return true
			}
			pcsiTracker.CtxSwitchInfo[cpu].Info = append(pcsiTracker.CtxSwitchInfo[cpu].Info, ppcsi)
		}
		return true
	}
	pcsiTracker.FilterFunc = filterFunc
	filter.AddFilter(filterFunc)
	return pcsiTracker
}
