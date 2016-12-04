package cpuprof

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToInt64(t *testing.T) {
	assert := assert.New(t)

	var err error
	var name string
	var bits int
	var str string
	var val int64

	name = "Convert-64-to-32"
	bits = 32
	str = "4294967296"
	val, err = StrToInt64(str, name, bits)
	assert.NotNil(err, "Converted 64bit integer into 32bit integer")
	val = strToInt64(str, name, bits)
	assert.Zero(val, "Converted 64bit integer into 32bit integer")

	name = "Convert-32-to-32"
	bits = 32
	str = "65536"
	val, err = StrToInt64(str, name, bits)
	assert.Equal(65536, int(val), "Bad parse")
	assert.Nil(err, "Failed to parse valid number")

	name = "Convert-64-to-64"
	bits = 64
	str = "4294967296"
	val, err = StrToInt64(str, name, bits)
	assert.Equal(int64(4294967296), val, "Bad parse")
	assert.Nil(err, "Failed to parse valid number")
}

func TestParseTraceFromLoglinePayload(t *testing.T) {
	var line string

	assert := assert.New(t)

	line = "dummy line"

	ti := ParseTraceFromLoglinePayload(nil)
	assert.Nil(ti, "Parsed a trace from nil logline")

	// Take a proper line and change the tag to something that will not be seen
	line = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_tempERATUR!#$@#: sensor_id=5 temp=32"
	logline := ParseLogline(line)
	assert.NotNil(logline, "Failed to parse valid line")
	ti = ParseTraceFromLoglinePayload(logline)
	assert.Nil(ti, "Correctly parsed an invalid trace event")

	// Mess around with the trace payload some more
	line = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [00a] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32"
	logline = ParseLogline(line)
	assert.NotNil(logline, "Failed to parse valid line")
	ti = ParseTraceFromLoglinePayload(logline)
	assert.Nil(ti, "Correctly parsed an invalid trace event")

	line = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] 20455.979145: thermal_temp: sensor_id=5 temp=32"
	logline = ParseLogline(line)
	assert.NotNil(logline, "Failed to parse valid line")
	ti = ParseTraceFromLoglinePayload(logline)
	assert.Nil(ti, "Correctly parsed an invalid trace event")

}

func TestCommonParse(t *testing.T) {
	assert := assert.New(t)

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu A online error=0"

	// The idea with this test is to get coverage over common errors that this function can throw
	// the per-trace error checking is done in each trace's corresponding test
	trace := NewTrace()
	ti := common_parse(str, SCHED_CPU_HOTPLUG_CONST, trace)
	assert.Nil(ti, "Correctly parsed bad trace line")

}

func TestParseSchedCpuHotplug(t *testing.T) {
	assert := assert.New(t)

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 1 online error=0"

	logline := ParseLogline(str)
	trace := ParseTraceFromLoglinePayload(logline)

	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("sched_cpu_hotplug", trace.Tag(), "Tag does not match")

	var sch *SchedCpuHotplug
	sch = trace.(*SchedCpuHotplug)

	assert.Equal(1, sch.Cpu, "Cpu parsing failed")
	assert.Equal("online", sch.State, "State parsing failed")
	assert.Equal(0, sch.Error, "Error parsing failed")

	str = "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 19999299009299 online error=0"
	logline = ParseLogline(str)
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Correctly parsed bad trace line")

	str = "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 1 online error=281283882183800"
	logline = ParseLogline(str)
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Correctly parsed bad trace line")

	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715869754   1461715869754.25        346fb177-c54f-4f8a-9385-124c461fd5cc    1269262 20458.403430    2016-04-27 00:11:09.754327      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] .n.2 20458.400542: phonelab_periodic_ctx_switch_info: cpu=0 pid=6440 tgid=6440 nice=0 comm=kworker/2:2 utime=0 stime=0 rtime=23281 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=1 s_int=0 s_unint=0 s_oth=1 log_idx=20457"

	logline = ParseLogline(str)
	assert.NotEqual(nil, logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("sched_cpu_hotplug", trace.Tag(), "Found sched_cpu_hotplug payload with non-sched_cpu_hotplug logline?")
}

func TestParseThermalTemp(t *testing.T) {
	assert := assert.New(t)

	// Valid
	str := "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32"
	logline := ParseLogline(str)
	trace := ParseTraceFromLoglinePayload(logline)
	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("thermal_temp", trace.Tag(), "Tag does not match")
	var tt *ThermalTemp
	tt = trace.(*ThermalTemp)
	assert.Equal(32, tt.Temp, "Temperature parsing failed")
	assert.Equal(5, tt.SensorId, "Sensor ID failed")

	// Invalid
	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=50042002040204 temp=32"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Correctly parsed bad line")

	// Invalid
	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32699699692939495"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Correctly parsed bad line")

	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715869754   1461715869754.25        346fb177-c54f-4f8a-9385-124c461fd5cc    1269262 20458.403430    2016-04-27 00:11:09.754327      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] .n.2 20458.400542: phonelab_periodic_ctx_switch_info: cpu=0 pid=6440 tgid=6440 nice=0 comm=kworker/2:2 utime=0 stime=0 rtime=23281 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=1 s_int=0 s_unint=0 s_oth=1 log_idx=20457"

	logline = ParseLogline(str)
	assert.NotEqual(nil, logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("periodic_ctx_switch_info", trace.Tag(), "Found periodic_ctx_switch_info payload with non-periodic_ctx_switch_info logline?")
}

func TestParseCpuFrequency(t *testing.T) {
	assert := assert.New(t)

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000 cpu_id=0"
	logline := ParseLogline(str)
	trace := ParseTraceFromLoglinePayload(logline)
	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("cpu_frequency", trace.Tag(), "Tag does not match")
	var cf *CpuFrequency
	cf = trace.(*CpuFrequency)
	assert.Equal(1728000, cf.State, "State parsing failed")
	assert.Equal(0, cf.CpuId, "CpuId failed")

	str = "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000000000 cpu_id=0"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Parsed invalid line correctly")

	str = "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000 cpu_id=993958293994529"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Parsed invalid line correctly")

	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715869754   1461715869754.25        346fb177-c54f-4f8a-9385-124c461fd5cc    1269262 20458.403430    2016-04-27 00:11:09.754327      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] .n.2 20458.400542: phonelab_periodic_ctx_switch_info: cpu=0 pid=6440 tgid=6440 nice=0 comm=kworker/2:2 utime=0 stime=0 rtime=23281 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=1 s_int=0 s_unint=0 s_oth=1 log_idx=20457"

	logline = ParseLogline(str)
	assert.NotEqual(nil, logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("cpu_frequency", trace.Tag(), "Found cpu_frequency payload with non-cpu_frequency logline?")
}

func TestParsePhonelabNumOnlineCpus(t *testing.T) {
	assert := assert.New(t)

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.26        29b2b79e-1a97-4f96-8070-7a26f952e92b    31951   1932.849450     2016-05-05 17:42:56.659837      216     216     D       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849100: phonelab_num_online_cpus: num_online_cpus=2"
	logline := ParseLogline(str)
	trace := ParseTraceFromLoglinePayload(logline)
	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("phonelab_num_online_cpus", trace.Tag(), "Tag does not match")
	var pnoc *PhonelabNumOnlineCpus
	pnoc = trace.(*PhonelabNumOnlineCpus)
	assert.Equal(2, pnoc.NumOnlineCpus, "NumOnlineCpus parsing failed")

	str = "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.26        29b2b79e-1a97-4f96-8070-7a26f952e92b    31951   1932.849450     2016-05-05 17:42:56.659837      216     216     D       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849100: phonelab_num_online_cpus: num_online_cpus=29994292949249"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Correctly parsed bad trace payload")

	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715869754   1461715869754.25        346fb177-c54f-4f8a-9385-124c461fd5cc    1269262 20458.403430    2016-04-27 00:11:09.754327      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] .n.2 20458.400542: phonelab_periodic_ctx_switch_info: cpu=0 pid=6440 tgid=6440 nice=0 comm=kworker/2:2 utime=0 stime=0 rtime=23281 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=1 s_int=0 s_unint=0 s_oth=1 log_idx=20457"

	logline = ParseLogline(str)
	assert.NotEqual(nil, logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("phonelab_num_online_cpus", trace.Tag(), "Found num online cpus payload with bad line?")
}

func TestParsePhonelabProcForeground(t *testing.T) {
	assert := assert.New(t)

	str := "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=894 comm=ndroid.systemui"
	logline := ParseLogline(str)
	_ = "breakpoint"
	trace := ParseTraceFromLoglinePayload(logline)
	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("phonelab_proc_foreground", trace.Tag(), "Tag does not match")
	var ppf *PhonelabProcForeground
	ppf = trace.(*PhonelabProcForeground)
	assert.Equal(894, ppf.Pid, "ProcForeground parsing failed")
	assert.Equal(894, ppf.Tgid, "ProcForeground parsing failed")
	assert.Equal("ndroid.systemui", ppf.Comm, "ProcForeground parsing failed")

	str = "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=89402104020400 tgid=894 comm=ndroid.systemui"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Parsed invalid line correctly")

	str = "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=8949492499249 comm=ndroid.systemui"
	logline = ParseLogline(str)
	assert.NotNil(logline, "Failed to parse valid line")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace, "Parsed invalid line correctly")

	str = "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715869754   1461715869754.25        346fb177-c54f-4f8a-9385-124c461fd5cc    1269262 20458.403430    2016-04-27 00:11:09.754327      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] .n.2 20458.400542: phonelab_periodic_ctx_switch_info: cpu=0 pid=6440 tgid=6440 nice=0 comm=kworker/2:2 utime=0 stime=0 rtime=23281 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=1 s_int=0 s_unint=0 s_oth=1 log_idx=20457"

	logline = ParseLogline(str)
	assert.NotEqual(nil, logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("phonelab_proc_foreground", trace.Tag(), "Found foreground with bad line?")
}

func TestParsePhonelabPeriodicCtxSwitchInfo(t *testing.T) {
	assert := assert.New(t)

	str := "e7a59f87-838c-48a0-bb82-bf97f5e8c79d 2016-07-05 14:19:10.239999912 82421087 [939984.528178]   201   201 D Kernel-Trace:      kworker/1:1-17    [001] ...2 939984.527405: phonelab_periodic_ctx_switch_info: cpu=1 pid=1563 tgid=1561 nice=0 comm=su_daemon utime=0 stime=0 rtime=318437 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=939983 rx=0 tx=0"
	logline := ParseLogline(str)
	trace := ParseTraceFromLoglinePayload(logline)

	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("phonelab_periodic_ctx_switch_info", trace.Tag(), "Tag does not match")

	var pcsi *PhonelabPeriodicCtxSwitchInfo
	pcsi = trace.(*PhonelabPeriodicCtxSwitchInfo)

	_ = "breakpoint"

	assert.Equal(1, pcsi.Cpu, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(1563, pcsi.Pid, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(1561, pcsi.Tgid, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(0, pcsi.Nice, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal("su_daemon", pcsi.Comm, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.Utime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.Stime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(318437), pcsi.Rtime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.BgUtime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.BgStime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.BgRtime, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.SRun, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(2), pcsi.SInt, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.SUnint, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.SOth, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(939983), pcsi.LogIdx, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.Rx, "PeriodicCtxSwitchInfo parsing failed")
	assert.Equal(int64(0), pcsi.Tx, "PeriodicCtxSwitchInfo parsing failed")

	str = "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D              Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=894 comm=ndroid.systemui"

	logline = ParseLogline(str)
	assert.NotNil(logline, "Parsing failed")
	trace = ParseTraceFromLoglinePayload(logline)
	assert.NotEqual("phonelab_periodic_ctx_switch_info", trace.Tag(), "Found ctx switch info with bad line?")
}

func TestParsePhonelabPeriodicCtxSwitchMarker(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	var ppcsm *PhonelabPeriodicCtxSwitchMarker

	// Fail state
	str := "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIAN cpu=1 count=0 log_idx=72"
	logline := ParseLogline(str)
	assert.NotNil(logline)
	trace := ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace)

	// Fail cpu
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=13030420520503023002 count=0 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	ppcsm = trace.(*PhonelabPeriodicCtxSwitchMarker)
	assert.Zero(ppcsm.Cpu, "Parsed illegal Cpu")

	// Negative cpu
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=-1 count=0 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace)

	// Fail count
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=1929319949124919491919 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	ppcsm = trace.(*PhonelabPeriodicCtxSwitchMarker)
	assert.Zero(ppcsm.Count, "Parsed illegal Count")
	// Negative count
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=-1 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace)

	// Fail log_idx
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=0 log_idx=95329539953923959235929359239293959599395935993939539593959199192191"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	ppcsm = trace.(*PhonelabPeriodicCtxSwitchMarker)
	assert.Zero(ppcsm.LogIdx, "Parsed illegal LogIdx")
	// Negative log_idx
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=0 log_idx=-44"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	assert.Nil(trace)

	// Valid
	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.375       0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9635    79.533367       2016-07-15 09:11:10.399999      202     202     D	       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.527064: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=369 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	ppcsm = trace.(*PhonelabPeriodicCtxSwitchMarker)
	assert.NotNil(ppcsm)
	assert.Equal(PPCSMBegin, ppcsm.State, "State did not match")
	assert.Equal(1, ppcsm.Cpu, "Cpu did not match")
	assert.Equal(369, ppcsm.Count, "Count did not match")
	assert.Equal(int64(72), ppcsm.LogIdx, "LogIdx did not match")

	str = "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.375       0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9635    79.533367       2016-07-15 09:11:10.399999      202     202     D	       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.527064: phonelab_periodic_ctx_switch_marker: END cpu=1 count=369 log_idx=72"
	logline = ParseLogline(str)
	assert.NotNil(logline)
	trace = ParseTraceFromLoglinePayload(logline)
	ppcsm = trace.(*PhonelabPeriodicCtxSwitchMarker)
	assert.NotNil(ppcsm)
	assert.Equal(PPCSMEnd, ppcsm.State, "State did not match")
	assert.Equal(1, ppcsm.Cpu, "Cpu did not match")
	assert.Equal(369, ppcsm.Count, "Count did not match")
	assert.Equal(int64(72), ppcsm.LogIdx, "LogIdx did not match")

}
