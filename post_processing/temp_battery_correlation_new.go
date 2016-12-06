package post_processing

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alecthomas/kingpin"
	"github.com/gurupras/go_cpuprof"
	"github.com/gurupras/go_cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons"
	"github.com/gurupras/gocommons/gsync"
)

const (
	SUSPEND_STRING string = "PM: suspend of devices complete"
)

var (
	app    *kingpin.Application
	reload *bool
	file   *bool
)

func setup_parser() *kingpin.Application {
	app = SetupParser()
	reload = app.Flag("reload", "Reload data instead of using dumps").Default("false").Bool()
	file = app.Flag("file", "Treat path as file").Default("false").Bool()
	return app
}

type TbcChunk struct {
	Start          *cpuprof.Healthd
	StartCpuState  map[string]*filters.CpuTrackerData
	StartFgBgState filters.FgBgState
	End            *cpuprof.Healthd
	Data           []*cpuprof.Logline
}

func NewTbcChunk() *TbcChunk {
	c := new(TbcChunk)
	return c
}

func GetDeviceTbcChunks(path, deviceId string, channel chan *TbcChunk) (err error) {
	defer close(channel)

	devicePath := filepath.Join(path, deviceId, "analysis", "temp_battery")

	var files []string
	if files, err = gocommons.ListFiles(devicePath, []string{"*.gz"}); err != nil {
		return err
	}

	var file *os.File
	var gzipReader *gzip.Reader

	idx := 0

	wg := sync.WaitGroup{}
	sem := gsync.NewSem(1)
	mutex := sync.Mutex{}
	for _, filePath := range files {
		wg.Add(1)
		sem.P()
		go func() {
			defer wg.Done()
			defer sem.V()
			buf := new(bytes.Buffer)
			if file, err = os.OpenFile(filePath, os.O_RDONLY, 0444); err != nil {
				err = errors.New(fmt.Sprintf("Failed to open file: %v  - %v", filePath, err))
				return
			}
			if gzipReader, err = gzip.NewReader(file); err != nil {
				err = errors.New(fmt.Sprintf("Failed to get gzip reader to: %v  - %v", filePath, err))
				return
			}
			if _, err = io.Copy(buf, gzipReader); err != nil {
				err = errors.New(fmt.Sprintf("Failed to copy bytes from gzipReader: %v", err))
				return
			}
			tbcChunk := new(TbcChunk)
			if err = json.Unmarshal(buf.Bytes(), tbcChunk); err != nil {
				err = errors.New(fmt.Sprintf("Failed to unmarshal file: %v  - %v", filePath, err))
				return
			}
			mutex.Lock()
			idx++
			//fmt.Println(fmt.Sprintf("Processed %d chunks", idx))
			mutex.Unlock()
			channel <- tbcChunk
		}()
	}
	wg.Wait()
	return
}

func assignToTbcChunk(chunk *TbcChunk, data []*cpuprof.Logline, healthd *cpuprof.Healthd, cpuStateMap map[string]*filters.CpuTrackerData, fgbgState filters.FgBgState) {
	if chunk.Start == nil {
		//fmt.Println("\tWriting start")
		chunk.Start = healthd
		data = data[:0]
		chunk.StartCpuState = make(map[string]*filters.CpuTrackerData)
		chunk.StartFgBgState = fgbgState
		for k, v := range cpuStateMap {
			chunk.StartCpuState[k] = new(filters.CpuTrackerData)
			*chunk.StartCpuState[k] = *v
		}
	} else if chunk.Start != nil {
		//fmt.Println("\tWriting end")
		chunk.Data = data
		/*
			//XXX: Debug code just to verify that data is in order
			var lastLogline *cpuprof.Logline = nil
			for _, logline := range data {
				if lastLogline != nil {
					if lastLogline.LogcatToken > logline.LogcatToken {
						fmt.Fprintln(os.Stderr, fmt.Sprintf("Loglines appearing out of order: \n%s\n%s\n", lastLogline.Line, logline.Line))
						os.Exit(-1)
					}
				}
				lastLogline = logline
			}
			// END OF DEBUG CODE
		*/
		chunk.End = healthd
	}
}

func tbcBootConsumer(bootid string, channel chan string, outChannel chan *TbcChunk) {
	defer close(outChannel)
	batteryChunk := NewTbcChunk()
	chunk_lines := make([]*cpuprof.Logline, 0)
	lines_processed := 0
	var line string
	var ok bool
	var healthd *cpuprof.Healthd = nil
	var is_healthd_line = false
	var recent_event bool = true
	var healthd_level int = -1
	var logline *cpuprof.Logline
	var lastLogline *cpuprof.Logline
	var last_healthd *cpuprof.Healthd

	cpuStateMap := make(map[string]*filters.CpuTrackerData, 0)
	filter := filters.New()
	cpuTracker := filters.NewCpuTracker(filter)
	cpuTrackerCallback := func(trace cpuprof.TraceInterface, cpu int) {
		cpuStateMap[fmt.Sprintf("%v", cpu)] = cpuTracker.CurrentState[cpu]
	}
	cpuTracker.Callback = cpuTrackerCallback

	fgbgTracker := filters.NewFgBgTracker(filter)

	filterList := filter.AsLineFilterArray()

	for {
		_ = last_healthd
		healthd = nil
		is_healthd_line = false

		if line, ok = <-channel; !ok {
			fmt.Println("Finished consuming bootid:", bootid)
			break
		}

		logline = cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}

		for _, f := range filterList {
			// XXX: Right now, no checks..
			// just running them through filter functions
			f(logline)
		}

		lines_processed++
		if lines_processed%100000 == 0 {
			//fmt.Println("tbcBootConsumer: Processed:", lines_processed)
		}
		// Make sure this logline's logcatToken is > previous
		if lastLogline != nil {
			if lastLogline.LogcatToken > logline.LogcatToken {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("Lines going backwards:\n%s\n%s\n", lastLogline.Line, logline.Line))
				os.Exit(-1)
			}
		}
		lastLogline = logline
		// Check for suspend
		/*
			if strings.Contains(logline.Payload, SUSPEND_STRING) {
				// This line contains suspend
				recent_event = true
				// Reset batteryChunk
				if batteryChunk.Start != nil {
					//fmt.Println("\tReset: Suspend")
					batteryChunk.Start = nil
					chunk_lines = make([]*cpuprof.Logline, 0)
				}
				continue
			}
		*/
		if strings.Compare(logline.Tag, "KernelPrintk") == 0 &&
			strings.Contains(logline.Payload, "healthd") &&
			strings.Contains(logline.Payload, "chg") {
			is_healthd_line = true
			healthd = cpuprof.ParseHealthdPrintk(logline)
		}
		if is_healthd_line {
			if recent_event {
				healthd_level = healthd.L
				recent_event = false
			}
			// Check if it is on charge
			if strings.Compare(healthd.Chg, "") != 0 {
				// It is on charge
				healthd_level = healthd.L
				// Clear batteryChunk
				if batteryChunk.Start != nil {
					chunk_lines = make([]*cpuprof.Logline, 0)
					batteryChunk.Start = nil
					//fmt.Println("\tReset: Charging")
				}
			} else {
				// It was not on charge
				if healthd_level != -1 {
					// We recently had a suspend/charge event
					if healthd.L <= healthd_level-1 {
						// We skipped 1 level after the suspend/charge event ended
						// So reset
						healthd_level = -1
						assignToTbcChunk(batteryChunk, chunk_lines, healthd, cpuStateMap, fgbgTracker.CurrentState)
						if batteryChunk.End != nil {
							// We have a full chunk
							chunk_lines = make([]*cpuprof.Logline, 0)
							outChannel <- batteryChunk
							// Now start the next one here
							batteryChunk = NewTbcChunk()
							assignToTbcChunk(batteryChunk, chunk_lines, healthd, cpuStateMap, fgbgTracker.CurrentState)
						}
					}
				} else {
					// This healthd log looks good
					if batteryChunk.Start != nil {
						if healthd.L == batteryChunk.Start.L {
							// They're equal. Just add this in there
							chunk_lines = append(chunk_lines, logline)
						} else if healthd.L == batteryChunk.Start.L-1 {
							// This is our end log
							assignToTbcChunk(batteryChunk, chunk_lines, healthd, cpuStateMap, fgbgTracker.CurrentState)
							chunk_lines = make([]*cpuprof.Logline, 0)
							outChannel <- batteryChunk
							batteryChunk = NewTbcChunk()
							// Now start the next one here
							assignToTbcChunk(batteryChunk, chunk_lines, healthd, cpuStateMap, fgbgTracker.CurrentState)
						}
					} else {
						assignToTbcChunk(batteryChunk, chunk_lines, healthd, cpuStateMap, fgbgTracker.CurrentState)
					}
				}
			}
			last_healthd = healthd
		} else {
			if batteryChunk.Start != nil {
				chunk_lines = append(chunk_lines, logline)
			}
		}
	}
}

type tbcFilter func(chunk *TbcChunk) bool

func saveTbcChunks(path string, device string, inChannel chan *TbcChunk, outChannel chan *TbcChunk) {
	defer close(outChannel)
	outdir := filepath.Join(path, device, DUMP_DIR, "temp_battery")

	idx := 0
	save := func(chunk *TbcChunk, idx int) {
		var err error
		var file *gocommons.File
		var writer gocommons.Writer

		filepath := filepath.Join(outdir, fmt.Sprintf("%08d.gz", idx))
		if file, err = gocommons.Open(filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, gocommons.GZ_TRUE); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to open file:", err)
			os.Exit(-1)
		}
		defer file.Close()
		if writer, err = file.Writer(0); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to get writer to file:", err)
			os.Exit(-1)
		}
		defer writer.Close()
		defer writer.Flush()

		var json_string []byte
		if json_string, err = json.MarshalIndent(chunk, "", "    "); err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to marshal:", err))
			os.Exit(-1)
		}
		writer.Write(json_string)
	}

	for {
		if chunk, ok := <-inChannel; !ok {
			break
		} else {
			save(chunk, idx)
			idx++
			outChannel <- chunk
		}
	}

}

func ProcessTempBatteryChunks(path string, device string, channel chan *TbcChunk, save bool) int {
	/**
	 * Because we get data from a channel and because we want to
	 * a) save it
	 * b) filter and process it
	 * we need to read from one channel and write it back to another
	 *
	 * Since we start with save, we will read from 'channel' and write to
	 * outChannel. Further stages of processing will read from this channel.
	 */
	saveOutChannel := make(chan *TbcChunk, 1)
	if *reload || save {
		// Serialize to file
		go saveTbcChunks(path, device, channel, saveOutChannel)
	} else {
		fmt.Fprintln(os.Stderr, "Warning: Passthrough")
		// Just pass through for further processing
		passThrough := func(inChannel, outChannel chan *TbcChunk) {
			outChannel <- <-inChannel
		}
		go passThrough(channel, saveOutChannel)
	}

	// Apply any necessary filters
	// A filter is expected to return true if it
	// meets all the criteria, else false
	suspend_filter := func(chunk *TbcChunk) bool {
		for _, logline := range chunk.Data {
			if strings.Contains(logline.Line, SUSPEND_STRING) {
				return false
			}
		}
		return true
	}

	missing_logline_filter := func(chunk *TbcChunk) bool {
		start_idx := chunk.Start.Logline.LogcatToken
		end_idx := chunk.End.Logline.LogcatToken

		current := start_idx + 1
		for idx, logline := range chunk.Data {
			if current+int64(idx) != logline.LogcatToken {
				return false
			}
		}
		if current+int64(len(chunk.Data)) != end_idx {
			return false
		}
		return true
	}
	_ = missing_logline_filter
	_ = suspend_filter
	filters := []tbcFilter{}

	// Now apply these filters to outChannel and write the output to filteredChannel
	filteredChannel := make(chan *TbcChunk, 1)

	filter_func := func(inChannel, outChannel chan *TbcChunk) {
		defer close(outChannel)
		var total int = 0
		var after_filter int = 0
		for {
			var chunk *TbcChunk
			var ok bool
			if chunk, ok = <-inChannel; !ok {
				break
			}
			total++
			pass := true
			for _, filter := range filters {
				pass = pass && filter(chunk)
				if !pass {
					break
				}
			}
			if pass {
				outChannel <- chunk
				after_filter++
			}
		}
		fmt.Println("total:", total, " filtered:", after_filter)
	}
	go filter_func(saveOutChannel, filteredChannel)

	// Now we have the output in filteredChannel. Do what you want with it.
	// Currently, we do nothing. We just discard everything and return
	nChunks := 0
	for {
		if nChunks%2 == 0 {
			//fmt.Println("Processed chunks:", nChunks)
		}

		if _, ok := <-filteredChannel; !ok {
			break
		}
		nChunks++
	}
	fmt.Println("ProcessTempBatteryChunks:", nChunks)
	return nChunks
}

func ProcessTBCBoot(boot *Boot, save bool) int {
	fmt.Println(fmt.Sprintf("Processing boot: %s -> %s", boot.DeviceId, boot.BootId))
	lineChannel := make(chan string, 100000)
	chunkChannel := make(chan *TbcChunk, 1)

	// Set up the boot reader
	go boot.AsyncRead(lineChannel)

	// Set up the boot consumer (and chunker)
	go tbcBootConsumer(boot.BootId, lineChannel, chunkChannel)

	// Set up consumer for chunks
	nChunks := ProcessTempBatteryChunks(boot.Path, boot.DeviceId, chunkChannel, save)
	fmt.Println("ProcessTBCBoot:", boot.BootId, ":", nChunks)
	return nChunks
}

func checkIfDone(dir string) bool {
	filePath := filepath.Join(dir, "meta.json")
	if exists, err := gocommons.Exists(filePath); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed while checking if done: %s:", filePath), err)
		os.Exit(-1)
	} else {
		return exists
	}
	return false
}

func TBCMain(args []string) {
	parser := setup_parser()
	ParseArgs(parser, args)

	if file != nil && *file == true {
		fmt.Fprintln(os.Stderr, "Unimplemented!")
		os.Exit(-1)
		if _, err := gocommons.Open(Path, os.O_RDONLY, gocommons.GZ_UNKNOWN); err != nil {
			os.Exit(-1)
		} else {
		}
	}

	device_files := GetDeviceFiles(Path, Devices)

	wg := sync.WaitGroup{}
	sem := gsync.NewSem(12)
	deviceCount := int32(0)
	processDevice := func(device string, boots []*Boot) {
		defer wg.Done()
		sem.P()
		defer sem.V()
		saveDir := filepath.Join(Path, device, DUMP_DIR, "temp_battery")
		var done bool = checkIfDone(saveDir)
		var save bool = !done
		if *reload {
			if done {
				os.RemoveAll(saveDir)
			}
			save = true
		}
		gocommons.Makedirs(saveDir)

		write_meta := func(device string, chunkMap map[string]int) {
			outFilePath := filepath.Join(saveDir, "meta.json")
			var outFile *gocommons.File
			var err error
			var writer gocommons.Writer

			if outFile, err = gocommons.Open(outFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, gocommons.GZ_FALSE); err != nil {
				fmt.Fprintln(os.Stderr, "Error while creating meta.json:", err)
				os.Exit(-1)
			}
			defer outFile.Close()

			if writer, err = outFile.Writer(0); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to open wrtier:", err)
				os.Exit(-1)
			}
			defer writer.Close()
			defer writer.Flush()

			var json_string []byte
			if json_string, err = json.MarshalIndent(chunkMap, "", "    "); err != nil {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to marshal:", err))
				os.Exit(-1)
			}
			writer.Write(json_string)
		}

		bootChunkMap := make(map[string]int)

		sem := gsync.NewSem(4)
		bootWg := sync.WaitGroup{}
		processBoot := func(boot *Boot, save bool) {
			defer sem.V()
			defer bootWg.Done()
			nChunks := ProcessTBCBoot(boot, save)
			fmt.Println("process_device:", boot.BootId, ":", nChunks)
			bootChunkMap[boot.BootId] = nChunks
		}

		for _, boot := range boots {
			sem.P()
			bootWg.Add(1)
			go processBoot(boot, save)
		}
		bootWg.Wait()
		// Now write the meta file
		write_meta(device, bootChunkMap)
		atomic.AddInt32(&deviceCount, int32(1))
		fmt.Println("Finished %d/%d", deviceCount, len(device_files))
	}

	for device, boots := range device_files {
		wg.Add(1)
		go processDevice(device, boots)
	}
	wg.Wait()
}
