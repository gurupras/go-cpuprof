package post_processing

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/set"
	"github.com/gurupras/gocommons"
)

const (
	DUMP_DIR            string = "analysis"
	THERMAL_PERIODS_DIR string = "thermal_periods"
)

var (
	App     *kingpin.Application
	path    *string
	Path    string
	regex   *string
	Regex   string
	device  *string
	Devices []string
	load    *bool
	Load    bool
	save    *bool
	Save    bool
)

func SetupParser() *kingpin.Application {
	App = kingpin.New("analysis", "")
	path = App.Arg("path", "").Required().String()
	regex = App.Flag("regex", "").Short('r').Default("*.out.gz").String()
	device = App.Flag("device", "").Short('d').String()
	load = App.Flag("load", "Load data from dump").Default("false").Bool()
	save = App.Flag("save", "Save data to dump").Default("false").Bool()
	return App
}

func ParseArgs(parser *kingpin.Application, args []string) {
	kingpin.MustParse(parser.Parse(args[1:]))

	// Now for the conversions
	Path = *path
	Regex = *regex
	Load = *load
	Save = *save

	if device != nil && strings.Compare(*device, "") != 0 {
		Devices = strings.Split(*device, " ")
	} else {
		Devices = make([]string, 0)
	}
}

func GetDevicesFiltered(path string, filter []string) []string {
	var err error
	var devices []string
	if devices, err = GetDevices(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return devices
	}
	if len(filter) > 0 {
		fmt.Println("filtering:", len(filter))
		devices_set := set.NewNonTS()
		for _, d := range devices {
			devices_set.Add(d)
		}
		filter_set := set.NewNonTS()
		for _, d := range filter {
			filter_set.Add(d)
		}
		devices = set.StringSlice(set.Intersection(devices_set, filter_set))
	}
	return devices
}

func Process(path string, devices []string, save bool) {
	device_files := GetDeviceFiles(path, devices)
	_ = device_files
}

func AnalysisMain(args []string) {
	parser := SetupParser()
	ParseArgs(parser, args)

	var files []string
	var devices []string
	var err error

	if files, err = gocommons.ListFiles(Path, []string{Regex}); err != nil {
		os.Exit(-1)
	}

	if Load {
		devices, _ = GetDevices(Path)
		_ = files
		_ = devices
		if device != nil {
			devices = Devices
		}
	}
	Process(Path, devices, Save)
}

func main() {
	AnalysisMain(os.Args)
}
