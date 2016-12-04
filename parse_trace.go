package cpuprof

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var TRACE_PATTERN = regexp.MustCompile(`` +
	`\s*(?P<thread>.*?)` +
	`\s+\[(?P<cpu>\d+)\]` +
	`\s+(?P<unknown>.{4})` +
	`\s+(?P<timestamp>\d+\.\d+)` +
	`: ` +
	`(?P<message>(?P<tag>.*?):` +
	`\s+(?P<text>.*)` +
	`)`)

type Trace struct {
	Thread    string
	Cpu       int
	Unknown   string
	Timestamp float64
	Tag       string
	Datetime  time.Time
	Logline   *Logline
}

func NewTrace() *Trace {
	t := new(Trace)
	return t
}

type TraceInterface interface {
	Tag() string
}

const (
	SCHED_CPU_HOTPLUG_CONST                   = 0
	THERMAL_TEMP_CONST                        = iota
	CPU_FREQUENCY_CONST                       = iota
	CPU_FREQUENCY_SWITCH_START_CONST          = iota
	CPU_FREQUENCY_SWITCH_END_CONST            = iota
	KGSL_GPUBUSY_CONST                        = iota
	KGSL_PWRLEVEL_CONST                       = iota
	PHONELAB_NUM_ONLINE_CPUS_CONST            = iota
	PHONELAB_PERIODIC_CTX_SWITCH_INFO_CONST   = iota
	PHONELAB_PERIODIC_CTX_SWITCH_MARKER_CONST = iota
	PHONELAB_PERIODIC_WARNING_CPU_CONST       = iota
	PHONELAB_TIMING_CONST                     = iota
	PHONELAB_PROC_FOREGROUND_CONST            = iota
	CPUFREQ_SCALING_CONST                     = iota
)

var ConstNames = [...]string{
	"SCHED_CPU_HOTPLUG_CONST",
	"THERMAL_TEMP_CONST",
	"CPU_FREQUENCY_CONST",
	"CPU_FREQUENCY_SWITCH_START_CONST",
	"CPU_FREQUENCY_SWITCH_END_CONST",
	"KGSL_GPUBUSY_CONST",
	"KGSL_PWRLEVEL_CONST",
	"PHONELAB_NUM_ONLINE_CPUS_CONST",
	"PHONELAB_PERIODIC_CTX_SWITCH_INFO_CONST",
	"PHONELAB_PERIODIC_CTX_SWITCH_MARKER_CONST",
	"PHONELAB_PERIODIC_WARNING_CPU_CONST",
	"PHONELAB_TIMING_CONST",
	"PHONELAB_PROC_FOREGROUND_CONST",
	"CPUFREQ_SCALING_CONST",
}

func StrToInt64(str string, name string, bits int) (int64, error) {
	var tmp int64
	var err error
	if tmp, err = strconv.ParseInt(str, 0, bits); err != nil {
		return tmp, err
	}
	return int64(tmp), err
}

func strToInt64(str string, name string, bits int) int64 {
	var tmp int64 = 0
	var err error
	if tmp, err = StrToInt64(str, name, bits); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to parse: '%s' (%s)", str, name))
		tmp = 0
	}
	return tmp
}

func ParseTraceFromLoglinePayload(logline *Logline) (ti TraceInterface) {
	if logline == nil {
		return nil
	}

	names := TRACE_PATTERN.SubexpNames()
	values_raw := TRACE_PATTERN.FindAllStringSubmatch(logline.Payload, -1)

	if values_raw == nil {
		return nil
	}

	values := values_raw[0]
	kv_map := map[string]string{}
	for i, value := range values {
		kv_map[names[i]] = value
	}

	trace := NewTrace()
	trace.Thread = kv_map["thread"]
	// The regex guarantees that these cannot fail
	trace.Cpu, _ = strconv.Atoi(kv_map["cpu"])

	trace.Unknown = kv_map["unknown"]

	trace.Timestamp, _ = strconv.ParseFloat(kv_map["timestamp"], 64)

	trace.Tag = kv_map["tag"]
	trace.Datetime = logline.Datetime

	// Uncomment this line if you want to add Logline information
	// Or add it manually where required
	//trace.Logline = logline

	switch trace.Tag {
	case "sched_cpu_hotplug":
		ti = sched_cpu_hotplug(kv_map["text"], trace)
	case "phonelab_num_online_cpus":
		ti = phonelab_num_online_cpus(kv_map["text"], trace)
	case "thermal_temp":
		ti = thermal_temp(kv_map["text"], trace)
	case "cpu_frequency":
		ti = cpu_frequency(kv_map["text"], trace)
	case "phonelab_proc_foreground":
		ti = phonelab_proc_foreground(kv_map["text"], trace)
	case "phonelab_periodic_ctx_switch_info":
		ti = phonelab_periodic_ctx_switch_info(kv_map["text"], trace)
	case "phonelab_periodic_ctx_switch_marker":
		ti = phonelab_periodic_ctx_switch_marker(kv_map["text"], trace)
	}
	return ti
}

func common_parse(text string, constant int, trace *Trace) (ti TraceInterface) {
	var err error
	var regex *regexp.Regexp
	dict := make(map[string]string)
	var names []string
	var values_raw [][]string
	switch constant {
	case SCHED_CPU_HOTPLUG_CONST:
		regex = SCHED_CPU_HOTPLUG_PATTERN
	case THERMAL_TEMP_CONST:
		regex = THERMAL_TEMP_PATTERN
	case CPU_FREQUENCY_CONST:
		regex = CPU_FREQUENCY_PATTERN
	case PHONELAB_NUM_ONLINE_CPUS_CONST:
		regex = PHONELAB_NUM_ONLINE_CPUS_PATTERN
	case PHONELAB_PROC_FOREGROUND_CONST:
		regex = PHONELAB_PROC_FOREGROUND_PATTERN
	case PHONELAB_PERIODIC_CTX_SWITCH_INFO_CONST:
		regex = PHONELAB_PERIODIC_CTX_SWITCH_INFO_PATTERN
	case PHONELAB_PERIODIC_CTX_SWITCH_MARKER_CONST:
		regex = PHONELAB_PERIODIC_CTX_SWITCH_MARKER_PATTERN
	}
	names = regex.SubexpNames()
	values_raw = regex.FindAllStringSubmatch(text, -1)

	if len(values_raw) == 0 {
		//fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to parse '%s' with '%s'", text, ConstNames[constant]))
		return nil
	}
	values := values_raw[0]
	for i, value := range values {
		dict[names[i]] = value
	}

	switch constant {
	case SCHED_CPU_HOTPLUG_CONST:
		sch := new(SchedCpuHotplug)
		sch.Trace = trace
		var cpu int64
		var state string
		var errorInt int64
		if cpu, err = strconv.ParseInt(dict["cpu"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse sensor_id")
			return nil
		}
		state = dict["state"]

		if errorInt, err = strconv.ParseInt(dict["error"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse temp")
			return nil
		}
		sch.Cpu = int(cpu)
		sch.State = state
		sch.Error = int(errorInt)
		ti = sch
	case THERMAL_TEMP_CONST:
		tt := NewThermalTemp()
		tt.Trace = trace
		var sensor int64
		var temp int64
		if sensor, err = strconv.ParseInt(dict["sensor_id"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse sensor_id")
			return nil
		}
		if temp, err = strconv.ParseInt(dict["temp"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse temp")
			return nil
		}
		tt.SensorId = int(sensor)
		tt.Temp = int(temp)
		ti = tt
	case CPU_FREQUENCY_CONST:
		cf := NewCpuFrequency()
		cf.Trace = trace
		var state int64
		var cpu_id int64
		if state, err = strconv.ParseInt(dict["state"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse state")
			return nil
		}
		if cpu_id, err = strconv.ParseInt(dict["cpu_id"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse cpu_id")
			return nil
		}
		cf.State = int(state)
		cf.CpuId = int(cpu_id)
		ti = cf
	case PHONELAB_NUM_ONLINE_CPUS_CONST:
		pnoc := new(PhonelabNumOnlineCpus)
		pnoc.Trace = trace
		var num_online_cpus int64
		if num_online_cpus, err = strconv.ParseInt(dict["num_online_cpus"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse num_online_cpus")
			return nil
		}
		pnoc.NumOnlineCpus = int(num_online_cpus)
		ti = pnoc
	case PHONELAB_PROC_FOREGROUND_CONST:
		ppf := new(PhonelabProcForeground)
		ppf.Trace = trace
		var tmp int64
		if tmp, err = strconv.ParseInt(dict["pid"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse pid")
			return nil
		}
		ppf.Pid = int(tmp)
		if tmp, err = strconv.ParseInt(dict["tgid"], 0, 32); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse tgid")
			return nil
		}
		ppf.Tgid = int(tmp)
		ppf.Comm = dict["comm"]
		ti = ppf
	case PHONELAB_PERIODIC_CTX_SWITCH_INFO_CONST:
		pcsi := new(PhonelabPeriodicCtxSwitchInfo)
		pcsi.Trace = trace
		pcsi.Cpu = int(strToInt64(dict["cpu"], "cpu", 32))
		pcsi.Pid = int(strToInt64(dict["pid"], "pid", 32))
		pcsi.Tgid = int(strToInt64(dict["tgid"], "tgid", 32))
		pcsi.Nice = int(strToInt64(dict["nice"], "nice", 32))
		pcsi.Comm = dict["comm"]
		pcsi.Utime = strToInt64(dict["utime"], "utime", 64)
		pcsi.Stime = strToInt64(dict["stime"], "stime", 64)
		pcsi.Rtime = strToInt64(dict["rtime"], "rtime", 64)
		pcsi.BgUtime = strToInt64(dict["bg_utime"], "bg_utime", 64)
		pcsi.BgStime = strToInt64(dict["bg_stime"], "bg_stime", 64)
		pcsi.BgRtime = strToInt64(dict["bg_rtime"], "bg_rtime", 64)
		pcsi.SRun = strToInt64(dict["s_run"], "s_run", 64)
		pcsi.SInt = strToInt64(dict["s_int"], "s_int", 64)
		pcsi.SUnint = strToInt64(dict["s_unint"], "s_unint", 64)
		pcsi.SOth = strToInt64(dict["s_oth"], "s_oth", 64)
		pcsi.LogIdx = strToInt64(dict["log_idx"], "log_idx", 64)
		if _, ok := dict["rx"]; ok && len(dict["rx"]) > 0 {
			pcsi.Rx = strToInt64(dict["rx"], "rx", 64)
			pcsi.Tx = strToInt64(dict["tx"], "tx", 64)
		} else {
			pcsi.Rx = 0
			pcsi.Tx = 0
		}
		ti = pcsi
	case PHONELAB_PERIODIC_CTX_SWITCH_MARKER_CONST:
		ppcsm := new(PhonelabPeriodicCtxSwitchMarker)
		ppcsm.Trace = trace
		ppcsm.State = PPCSMState(dict["state"])
		ppcsm.Cpu = int(strToInt64(dict["cpu"], "cpu", 32))
		ppcsm.Count = int(strToInt64(dict["count"], "count", 32))
		ppcsm.LogIdx = strToInt64(dict["log_idx"], "log_idx", 64)
		ti = ppcsm
	}
	return ti
}

/* Format: cpu 1 offline error=0 */
var SCHED_CPU_HOTPLUG_PATTERN = regexp.MustCompile(`` +
	`\s*cpu` +
	`\s+(?P<cpu>\d+)` +
	`\s+(?P<state>[a-zA-Z0-9_]+)` +
	`\s+error=(?P<error>-?\d+)`)

type SchedCpuHotplug struct {
	Trace *Trace
	Cpu   int
	State string
	Error int
}

func (t *SchedCpuHotplug) Tag() string {
	return t.Trace.Tag
}

func sched_cpu_hotplug(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, SCHED_CPU_HOTPLUG_CONST, trace)
	return obj
}

/* Format: sensor_id=5 temp=59 */
var THERMAL_TEMP_PATTERN = regexp.MustCompile(`` +
	`\s*sensor_id=(?P<sensor_id>\d+)` +
	`\s+temp=(?P<temp>\d+).*`)

type ThermalTemp struct {
	Trace    *Trace
	SensorId int
	Temp     int
}

func NewThermalTemp() *ThermalTemp {
	tt := new(ThermalTemp)
	return tt
}

func (t *ThermalTemp) Tag() string {
	return t.Trace.Tag
}

func thermal_temp(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, THERMAL_TEMP_CONST, trace)
	return obj
}

/* Format: cpu_frequency: state=2265600 cpu_id=0 */
var CPU_FREQUENCY_PATTERN = regexp.MustCompile(`` +
	`\s*state=(?P<state>\d+)` +
	`\s+cpu_id=(?P<cpu_id>\d+)`)

type CpuFrequency struct {
	Trace *Trace
	State int
	CpuId int
}

func NewCpuFrequency() *CpuFrequency {
	cf := new(CpuFrequency)
	return cf
}

func (cf *CpuFrequency) Tag() string {
	return cf.Trace.Tag
}

func cpu_frequency(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, CPU_FREQUENCY_CONST, trace)
	return obj
}

/* Format: phonelab_num_online_cpus: num_online_cpus=4 */
var PHONELAB_NUM_ONLINE_CPUS_PATTERN = regexp.MustCompile(`` +
	`\s*num_online_cpus=(?P<num_online_cpus>\d+)`)

type PhonelabNumOnlineCpus struct {
	Trace         *Trace
	NumOnlineCpus int
}

func (ti *PhonelabNumOnlineCpus) Tag() string {
	return ti.Trace.Tag
}

func phonelab_num_online_cpus(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, PHONELAB_NUM_ONLINE_CPUS_CONST, trace)
	return obj
}

/* Format: phonelab_proc_foreground: pid=13759 tgid=13759 comm=.android.dialer */
var PHONELAB_PROC_FOREGROUND_PATTERN = regexp.MustCompile(`` +
	`\s*pid=(?P<pid>\d+) tgid=(?P<tgid>\d+) comm=(?P<comm>\S+)`)

type PhonelabProcForeground struct {
	Trace *Trace
	Pid   int
	Tgid  int
	Comm  string
}

func (ti *PhonelabProcForeground) Tag() string {
	return ti.Trace.Tag
}

func phonelab_proc_foreground(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, PHONELAB_PROC_FOREGROUND_CONST, trace)
	return obj
}

var PHONELAB_PERIODIC_CTX_SWITCH_MARKER_PATTERN = regexp.MustCompile(`` +
	`\s*(?P<state>BEGIN|END)` +
	`\s+cpu=(?P<cpu>\d+)` +
	`\s+count=(?P<count>\d+)` +
	`\s+log_idx=(?P<log_idx>\d+)`)

type PPCSMState string

const (
	PPCSMBegin PPCSMState = "BEGIN"
	PPCSMEnd   PPCSMState = "END"
)

type PhonelabPeriodicCtxSwitchMarker struct {
	Trace  *Trace
	State  PPCSMState
	Cpu    int
	Count  int
	LogIdx int64
}

func (ti *PhonelabPeriodicCtxSwitchMarker) Tag() string {
	return ti.Trace.Tag
}

func phonelab_periodic_ctx_switch_marker(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, PHONELAB_PERIODIC_CTX_SWITCH_MARKER_CONST, trace)
	return obj
}

/* Format: phonelab_periodic_ctx_switch_info: cpu=0 pid=3 tgid=3 nice=0 comm=ksoftirqd/0 utime=0 stime=0 rtime=1009429 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=17 s_unint=0 s_oth=0 log_idx=933300 rx=0 tx=0 */
var PHONELAB_PERIODIC_CTX_SWITCH_INFO_PATTERN = regexp.MustCompile(`` +
	`\s*cpu=(?P<cpu>\d+)` +
	`\s*pid=(?P<pid>\d+)` +
	`\s*tgid=(?P<tgid>\d+)` +
	`\s*nice=(?P<nice>-?\d+)` +
	`\s*comm=(?P<comm>.*?)` +
	`\s*utime=(?P<utime>\d+)` +
	`\s*stime=(?P<stime>\d+)` +
	`\s*rtime=(?P<rtime>\d+)` +
	`\s*bg_utime=(?P<bg_utime>\d+)` +
	`\s*bg_stime=(?P<bg_stime>\d+)` +
	`\s*bg_rtime=(?P<bg_rtime>\d+)` +
	`\s*s_run=(?P<s_run>\d+)` +
	`\s*s_int=(?P<s_int>\d+)` +
	`\s*s_unint=(?P<s_unint>\d+)` +
	`\s*s_oth=(?P<s_oth>\d+)` +
	`\s*log_idx=(?P<log_idx>\d+)` +
	`(` +
	`\s*rx=(?P<rx>\d+)` +
	`\s*tx=(?P<tx>\d+)` +
	`)?`)

type PhonelabPeriodicCtxSwitchInfo struct {
	Trace   *Trace
	Cpu     int
	Pid     int
	Tgid    int
	Nice    int
	Comm    string
	Utime   int64
	Stime   int64
	Rtime   int64
	BgUtime int64
	BgStime int64
	BgRtime int64
	SRun    int64
	SInt    int64
	SUnint  int64
	SOth    int64
	LogIdx  int64
	Rx      int64
	Tx      int64
}

func (ti *PhonelabPeriodicCtxSwitchInfo) Tag() string {
	return ti.Trace.Tag
}

func phonelab_periodic_ctx_switch_info(text string, trace *Trace) TraceInterface {
	obj := common_parse(text, PHONELAB_PERIODIC_CTX_SWITCH_INFO_CONST, trace)
	return obj
}

type PeriodicCtxSwitchInfo struct {
	Start *PhonelabPeriodicCtxSwitchMarker
	Info  []*PhonelabPeriodicCtxSwitchInfo
	End   *PhonelabPeriodicCtxSwitchMarker
}

func (pcsi *PeriodicCtxSwitchInfo) TotalTime() int64 {
	total_time := int64(0)
	for _, info := range pcsi.Info {
		total_time += info.Rtime
	}
	return total_time
}

func (pcsi *PeriodicCtxSwitchInfo) Busyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) FgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
			busy_time -= info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) BgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}
