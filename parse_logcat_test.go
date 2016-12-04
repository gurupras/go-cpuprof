package cpuprof

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/gurupras/gocommons"
	"github.com/jehiah/go-strftime"
	"github.com/stretchr/testify/assert"
)

func TestCheckLogcatPattern(t *testing.T) {
	assert := assert.New(t)

	line := "6b793913-7cd9-477a-bbfa-62f07fbac87b 2016-04-21 09:59:01.199025638 11553177 [29981.752359]   202   203 D Kernel-Trace:      kworker/1:1-21588 [001] ...2 29981.751893: phonelab_periodic_ctx_switch_info: cpu=1 pid=7641 tgid=7613 nice=0 comm=Binder_1 utime=0 stime=0 rtime=158906 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=79981"

	logline := ParseLogline(line)
	assert.NotNil(logline, "Failed to parse logline")

	assert.Equal("6b793913-7cd9-477a-bbfa-62f07fbac87b", logline.BootId, "BootId does not match")
	assert.Equal("2016-04-21 09:59:01", strftime.Format("%Y-%m-%d %H:%M:%S", logline.Datetime), "Datetime does not match")
	assert.Equal(int64(199025638), logline.DatetimeNanos, "DatetimeNanos does not match")
	assert.Equal(int64(11553177), logline.LogcatToken, "LogcatToken does not match")
	assert.Equal(29981.752359, logline.TraceTime, "TraceTime does not match")
	assert.Equal(int32(202), logline.Pid, "Pid does not match")
	assert.Equal(int32(203), logline.Tid, "Tid does not match")
	assert.Equal("D", logline.Level, "Level does not match")
	assert.Equal("Kernel-Trace", logline.Tag, "Tag does not match")

	payload := "kworker/1:1-21588 [001] ...2 29981.751893: phonelab_periodic_ctx_switch_info: cpu=1 pid=7641 tgid=7613 nice=0 comm=Binder_1 utime=0 stime=0 rtime=158906 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=79981"
	assert.Equal(payload, logline.Payload, "Payload does not match")
}

func TestCheckLogcatSort(t *testing.T) {
	var infile_raw *gocommons.File
	var err error
	var reader *bufio.Scanner

	assert := assert.New(t)

	if infile_raw, err = gocommons.Open("./test", os.O_RDONLY, gocommons.GZ_FALSE); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to open:", "./test", ":", err)
		return
	}
	defer infile_raw.Close()
	if reader, err = infile_raw.Reader(0); err != nil {
		fmt.Fprintln(os.Stderr, "Could not get reader:", "./test")
		return
	}

	reader.Split(bufio.ScanLines)
	loglines := make(gocommons.SortCollection, 0)
	for reader.Scan() {
		line := reader.Text()
		logline := ParseLogline(line)
		if logline != nil {
			loglines = append(loglines, logline)
		}
	}
	sort.Sort(loglines)

	logIdx := (loglines[0].(*Logline)).LogcatToken - 1
	for idx, l := range loglines {
		logline := (l.(*Logline))
		assert.Equal(logIdx+int64(idx+1), logline.LogcatToken, "Tokens don't match")
	}
}

func TestParseLoglineConvert(t *testing.T) {
	assert := assert.New(t)

	line := "6b793913-7cd9-477a-bbfa-62f07fbac87b 2016-04-21 09:59:01.199025638 11553177 [29981.752359]   202   203 D Kernel-Trace:      kworker/1:1-21588 [001] ...2 29981.751893: phonelab_periodic_ctx_switch_info: cpu=1 pid=7641 tgid=7613 nice=0 comm=Binder_1 utime=0 stime=0 rtime=158906 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=79981"

	sortInterface := ParseLoglineConvert(line)
	assert.NotNil(sortInterface, "Failed to parse valid line into gocommons.SortInterface")

	// Now for the negative test
	line = "dummy string"
	sortInterface = ParseLoglineConvert(line)
	assert.Nil(sortInterface, "Converted invalid line into gocommons.SortInterface")
}

func TestLoglineString(t *testing.T) {
	assert := assert.New(t)

	line := "6b793913-7cd9-477a-bbfa-62f07fbac87b 2016-04-21 09:59:01.199025638 11553177 [29981.752359]   202   203 D Kernel-Trace:      kworker/1:1-21588 [001] ...2 29981.751893: phonelab_periodic_ctx_switch_info: cpu=1 pid=7641 tgid=7613 nice=0 comm=Binder_1 utime=0 stime=0 rtime=158906 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=79981"

	logline := ParseLogline(line)
	assert.Equal(line, logline.String(), "Lines did not match")
}
