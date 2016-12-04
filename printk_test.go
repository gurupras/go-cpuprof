package cpuprof

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMsmThermalPrintk(t *testing.T) {
	str := "6890aa2f-9895-47bf-9c37-79a2e3a34703 2016-06-25 13:24:51.291000001 3325 [   21.522780]   200   200 D KernelPrintk: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66"

	logline := ParseLogline(str)
	mtp := ParseMsmThermalPrintk(logline)

	assert.NotEqual(t, nil, mtp, "Parsing msm_thermal failed")
	assert.Equal(t, MSM_THERMAL_STATE_ONLINE, mtp.State, "State parsing failed")
	assert.Equal(t, 3, mtp.Cpu, "CPU parsing failed")
	assert.Equal(t, 66, mtp.Temp, "Temperature parsing failed")

	str = "6890aa2f-9895-47bf-9c37-79a2e3a34703 1970-06-07 17:06:20.399999996 2981 [   18.272750]   200   200 D KernelPrintk: <6>[   18.262644] msm_thermal: Set Offline: CPU2 Temp: 80"

	logline = ParseLogline(str)
	mtp = ParseMsmThermalPrintk(logline)

	assert.NotEqual(t, nil, mtp, "Parsing msm_thermal failed")
	assert.Equal(t, MSM_THERMAL_STATE_OFFLINE, mtp.State, "State parsing failed")
	assert.Equal(t, 2, mtp.Cpu, "CPU parsing failed")
	assert.Equal(t, 80, mtp.Temp, "Temperature parsing failed")

}
