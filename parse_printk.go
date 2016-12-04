package cpuprof

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pbnjay/strptime"
)

type MsmThermalState int

const (
	MSM_THERMAL_STATE_OFFLINE MsmThermalState = iota
	MSM_THERMAL_STATE_ONLINE  MsmThermalState = iota
)

type PowerManagementState int

const (
	PM_SUSPEND_ENTRY PowerManagementState = iota
	PM_SUSPEND_EXIT  PowerManagementState = iota
)

type MsmThermalPrintk struct {
	Logline  *Logline
	StateStr string
	State    MsmThermalState
	Cpu      int
	Temp     int
}

/* Printk pattern example: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66 */
var PRINTK_PATTERN_STRING = `(<(?P<loglevel>\d+)>)?\s*\[\s*(?P<timestamp>\d+\.\d+)\]\s*`
var MSM_THERMAL_PRINTK_PATTERN = regexp.MustCompile(PRINTK_PATTERN_STRING +
	`\s*msm_thermal: (?P<state>(Set Offline:|Allow Online)) CPU(?P<cpu>\d+) Temp: (?P<temp>\d+)`)

func ParseMsmThermalPrintk(logline *Logline) *MsmThermalPrintk {
	var err error

	names := MSM_THERMAL_PRINTK_PATTERN.SubexpNames()
	values_raw := MSM_THERMAL_PRINTK_PATTERN.FindAllStringSubmatch(logline.Payload, -1)
	if values_raw == nil {
		//fmt.Fprintln(os.Stderr, "Failed to parse:", logline.Payload)
		return nil
	}

	values := values_raw[0]

	kv_map := map[string]string{}

	for i, value := range values {
		kv_map[names[i]] = value
	}

	mtp := new(MsmThermalPrintk)

	mtp.Logline = logline
	mtp.StateStr = kv_map["state"]
	if strings.Compare(mtp.StateStr, "Set Offline:") == 0 {
		mtp.State = MSM_THERMAL_STATE_OFFLINE
	} else if strings.Compare(mtp.StateStr, "Allow Online") == 0 {
		mtp.State = MSM_THERMAL_STATE_ONLINE
	} else {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Unknown msm_thermal state '%s'", mtp.StateStr))
		os.Exit(-1)
	}
	var cpu int64
	var temp int64
	if cpu, err = strconv.ParseInt(kv_map["cpu"], 0, 32); err != nil {
		return nil
	}
	mtp.Cpu = int(cpu)

	if temp, err = strconv.ParseInt(kv_map["temp"], 0, 32); err != nil {
		return nil
	}
	mtp.Temp = int(temp)

	return mtp
}

type PowerManagementPrintk struct {
	Logline  *Logline
	State    PowerManagementState
	Datetime time.Time
}

/* Format:
<6>[93341.687692] PM: suspend exit 2016-04-27 04:00:00.220795560 UTC
<6>[93341.915138] PM: suspend entry 2016-04-27 04:00:00.448241184 UTC
*/

var POWER_MANAGEMENT_PRINTK_PATTERN = regexp.MustCompile(PRINTK_PATTERN_STRING +
	`\s*PM: suspend (?P<state>entry|exit) (?P<datetime>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.\d+) (?P<timezone>\S+)`)

func ParsePowerManagementPrintk(logline *Logline) *PowerManagementPrintk {
	var err error

	names := POWER_MANAGEMENT_PRINTK_PATTERN.SubexpNames()
	values_raw := POWER_MANAGEMENT_PRINTK_PATTERN.FindAllStringSubmatch(logline.Payload, -1)
	if values_raw == nil {
		//fmt.Fprintln(os.Stderr, "Failed to parse:", logline.Payload)
		return nil
	}

	values := values_raw[0]

	kv_map := map[string]string{}

	for i, value := range values {
		kv_map[names[i]] = value
	}
	pmp := new(PowerManagementPrintk)

	pmp.Logline = logline

	if strings.Compare(kv_map["state"], "entry") == 0 {
		pmp.State = PM_SUSPEND_ENTRY
	} else if strings.Compare(kv_map["state"], "exit") == 0 {
		pmp.State = PM_SUSPEND_EXIT
	} else {
		fmt.Fprintln(os.Stderr, "Unknown pm state for line:", logline.Line)
		os.Exit(-1)
	}

	datetime, err := strptime.Parse(kv_map["datetime"], "%Y-%m-%d %H:%M:%S.%f")
	if err != nil {
		return nil
	}
	pmp.Datetime = datetime
	return pmp
}

var HEALTHD_PATTERN = regexp.MustCompile(`` +
	PRINTK_PATTERN_STRING +
	`healthd:\s*battery\s*l=(?P<l>\d+)\s+v=(?P<v>\d+)\s+t=(?P<t>\d+\.\d+)\s+h=(?P<h>-?\d+)\s+st=(?P<st>-?\d+)\s+c=(?P<c>-?\d+)\s+chg=(?P<chg>([auw]+)?)\s*`)

type Healthd struct {
	Logline   *Logline
	Timestamp float64
	L         int
	V         int
	T         float64
	H         int
	St        int
	C         int
	Chg       string
}

func ParseHealthdPrintk(logline *Logline) *Healthd {
	names := HEALTHD_PATTERN.SubexpNames()
	values_raw := HEALTHD_PATTERN.FindAllStringSubmatch(logline.Payload, -1)
	if len(values_raw) == 0 {
		fmt.Fprintln(os.Stderr, "Failed to parse:", logline.Line)
		os.Exit(-1)
	}
	values := values_raw[0]

	kv_map := map[string]string{}
	for i, value := range values {
		kv_map[names[i]] = value
	}
	timestamp, err := strconv.ParseFloat(kv_map["timestamp"], 64)
	if err != nil {
		return nil
	}
	l, err := strconv.ParseInt(kv_map["l"], 0, 32)
	if err != nil {
		return nil
	}
	v, err := strconv.ParseInt(kv_map["v"], 0, 32)
	if err != nil {
		return nil
	}
	t, err := strconv.ParseFloat(kv_map["t"], 64)
	if err != nil {
		return nil
	}
	h, err := strconv.ParseInt(kv_map["h"], 0, 32)
	if err != nil {
		return nil
	}
	st, err := strconv.ParseInt(kv_map["st"], 0, 32)
	if err != nil {
		return nil
	}
	c, err := strconv.ParseInt(kv_map["c"], 0, 32)
	if err != nil {
		return nil
	}
	chg := kv_map["chg"]

	obj := Healthd{logline, timestamp, int(l), int(v), t, int(h), int(st), int(c), chg}
	return &obj
}

var PVS_BIN_PATTERN = regexp.MustCompile(`` +
	PRINTK_PATTERN_STRING +
	`acpuclk-8974 qcom,acpuclk.30: ACPU PVS: (?P<pvs_bin>\d+)`)

type PvsBin struct {
	Logline   *Logline
	Timestamp float64
	PvsBin    int
}

func ParsePvsBin(logline *Logline) *PvsBin {
	names := PVS_BIN_PATTERN.SubexpNames()
	values_raw := PVS_BIN_PATTERN.FindAllStringSubmatch(logline.Payload, -1)
	if len(values_raw) == 0 {
		fmt.Fprintln(os.Stderr, "Failed to parse:", logline.Line)
		os.Exit(-1)
	}
	values := values_raw[0]

	kv_map := map[string]string{}
	for i, value := range values {
		kv_map[names[i]] = value
	}
	timestamp, err := strconv.ParseFloat(kv_map["timestamp"], 64)
	if err != nil {
		return nil
	}
	pvsBin, err := strconv.ParseInt(kv_map["pvs_bin"], 0, 32)
	if err != nil {
		return nil
	}
	obj := PvsBin{logline, timestamp, int(pvsBin)}
	return &obj
}
