package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gurupras/go_cpuprof"
	"github.com/gurupras/go_cpuprof/post_processing"
	"github.com/gurupras/go_cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons"
)

func deviceProducer(path, device string, channel chan *cpuprof.Logline, filters []filters.LineFilter, signal *sync.WaitGroup) {
	//fmt.Printf("deviceProducer signal:%p\n", signal)
	defer signal.Done()
	defer close(channel)

	lines_read := 0
	fileReaderIdx := 0
	fileReader := func(idx int, path string, channel chan *cpuprof.Logline, filters []filters.LineFilter, wg *sync.WaitGroup) {
		//fmt.Printf("fileReader wg:%p\n", wg)
		defer wg.Done()
		//fmt.Println("Starting fileReader:", idx)
		var err error
		var fstruct *gocommons.File
		var reader *bufio.Scanner
		if fstruct, err = gocommons.Open(path, os.O_RDONLY, gocommons.GZ_TRUE); err != nil {
			return
		}
		defer fstruct.Close()
		if reader, err = fstruct.Reader(0); err != nil {
			return
		}

		reader.Split(bufio.ScanLines)
		var passed bool = true
		num_channel_write := 0
		for reader.Scan() {
			line := reader.Text()
			lines_read++
			if lines_read%100000 == 0 {
				//fmt.Printf("%s - %d\n", device, lines_read)
			}
			passed = true
			for _, filter := range filters {
				passed = passed && filter(line)
				if !passed {
					break
				}
			}
			if !passed {
				continue
			}
			logline := cpuprof.ParseLogline(line)
			channel <- logline
			num_channel_write++
		}
		//fmt.Println("Ending fileReader:", idx)
	}

	var err error
	var files []string
	wg := sync.WaitGroup{}
	device_path := filepath.Join(path, device)

	//fmt.Printf("Starting Producer-%s\n", device)
	//fmt.Printf("deviceConsumer - filereader wg:%p\n", &wg)
	if files, err = gocommons.ListFiles(device_path, []string{"*.out.gz"}); err != nil {
		fmt.Fprintln(os.Stderr, "Could not list files")
		os.Exit(-1)
	}
	for _, file := range files {
		wg.Add(1)
		go fileReader(fileReaderIdx, file, channel, filters, &wg)
		fileReaderIdx++
	}
	//fmt.Println("Waiting for consumers...")
	wg.Wait()
	//fmt.Println(fmt.Sprintf("Producer-%s: Done", device))
}

func deviceConsumer(path string, device string, channel chan *cpuprof.Logline, signal *sync.WaitGroup) {
	//fmt.Printf("deviceConsumer signal:%p\n", signal)
	defer signal.Done()
	//fmt.Printf("Starting Consumer-%s\n", device)

	dump_dir := filepath.Join(path, device, post_processing.DUMP_DIR)
	gocommons.Makedirs(dump_dir)
	var fstruct *gocommons.File
	var writer gocommons.Writer
	var err error
	file := filepath.Join(dump_dir, "temps.gz")
	if fstruct, err = gocommons.Open(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, gocommons.GZ_TRUE); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	defer fstruct.Close()
	if writer, err = fstruct.Writer(0); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	defer writer.Close()
	defer writer.Flush()

	traces_dumped := 0
	for {
		if logline, ok := <-channel; !ok {
			break
		} else {
			trace := cpuprof.ParseTraceFromLoglinePayload(logline)
			if trace != nil && strings.Compare(trace.Tag(), "thermal_temp") == 0 {
				if traces_dumped != 0 {
					writer.Write([]byte("\n"))
				}
				writer.Write([]byte(logline.Line))
				traces_dumped += 1
			}
		}
	}
	fmt.Printf("%s-%d\n", device, traces_dumped)
	//fmt.Println(fmt.Sprintf("Consumer-%s: Done", device))
}

func stringFilter(line string) bool {
	if strings.Contains(line, "thermal_temp") {
		return true
	} else {
		return false
	}
}

func main() {
	app := post_processing.SetupParser()
	post_processing.ParseArgs(app, os.Args)

	devices := post_processing.GetDevicesFiltered(post_processing.Path, post_processing.Devices)
	fmt.Println("devices:", devices)

	device_channel_map := make(map[string]chan *cpuprof.Logline)

	filters := []filters.LineFilter{stringFilter}
	wg := sync.WaitGroup{}
	//fmt.Printf("Main wg:%p\n", &wg)
	for _, d := range devices {
		device_channel_map[d] = make(chan *cpuprof.Logline, 100)
		wg.Add(2)
		go deviceProducer(post_processing.Path, d, device_channel_map[d], filters, &wg)
		go deviceConsumer(post_processing.Path, d, device_channel_map[d], &wg)
	}
	wg.Wait()
}
