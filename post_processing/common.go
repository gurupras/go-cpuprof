package post_processing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/set"
	"github.com/google/shlex"
	"github.com/gurupras/cpuprof"
	"github.com/gurupras/gocommons"
)

func GetDevices(path string) (devices []string, err error) {
	cmd := fmt.Sprintf("ls -d %s/*/ | grep -P '%s/[a-f0-9]{40}/'", path, path)
	args, _ := shlex.Split(cmd)
	if ret, stdout, stderr := gocommons.Execv(args[0], args[1:], true); ret != 0 {
		err = errors.New(fmt.Sprintf("Failed to run program:%v", stderr))
		return
	} else {
		dirs := strings.Split(stdout, "\n")
		for _, dir := range dirs {
			if len(strings.TrimSpace(dir)) == 0 {
				continue
			}
			basename := filepath.Base(dir)
			devices = append(devices, basename)
		}
	}
	return
}

func GetDeviceFiles(path string, filter []string) map[string][]*Boot {
	devices, _ := GetDevices(path)

	device_files := make(map[string][]*Boot)

	if len(filter) > 0 {
		ds := set.NewNonTS()
		for _, d := range devices {
			ds.Add(d)
		}

		dfs := set.NewNonTS()
		for _, d := range filter {
			dfs.Add(d)
		}

		devices = set.StringSlice(set.Intersection(ds, dfs))
	}

	skipped := 0
	for _, d := range devices {
		if _, ok := device_files[d]; !ok {
			device_files[d] = make([]*Boot, 0)
		}
		dpath := filepath.Join(path, d)
		info := make(map[string][]string)
		var err error

		if info, err = cpuprof.GetInfo(dpath); err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Warning: No info found for device:%v...skipping", d))
			skipped++
			continue
			//os.Exit(-1)
		}
		bootids := info["bootids"]
		for _, bootidStr := range bootids {
			boot := NewBoot(path, d, bootidStr)
			device_files[d] = append(device_files[d], boot)
		}
	}
	if skipped > 0 {
		fmt.Fprintln(os.Stderr, "Skipped devices: ", skipped)
	}
	return device_files
}
