package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/alecthomas/kingpin"
	"github.com/gurupras/go_cpuprof"
	"github.com/gurupras/gocommons"
	"github.com/gurupras/gocommons/gsync"
)

var (
	kpin  = kingpin.New("stitch", "")
	path  = kpin.Arg("path", "").Required().String()
	regex = kpin.Flag("regex", "").Short('r').Default("*.out.gz").String()
)

var (
	logger *logrus.Logger
)

type DeviceData struct {
	*CommControl
	DeviceFileChannel chan string
	PvsChannel        chan *Pvs
}

type CommControl struct {
	Channel chan struct{}
	sync.Mutex
	Closed bool
}

type Pvs struct {
	DeviceId string
	PvsBin   int
}

var started int = 0
var done int = 0

func ProcessPvs(deviceId string, file string, commControl *CommControl, pvsChannel chan *Pvs) {
	var fstruct *gocommons.File
	var reader *bufio.Scanner
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%s - %s: Failed: %v", deviceId, file, r))
		}
	}()

	logger.Infoln(fmt.Sprintf("%s - %s", deviceId, file))

	if fstruct, err = gocommons.Open(file, os.O_RDONLY, gocommons.GZ_TRUE); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to open: %v: %v", file, err))
		return
	}

	if reader, err = fstruct.Reader(1048576); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to get reader to: %v: %v", fstruct.Path, err))
		return
	}

	reader.Split(bufio.ScanLines)

	idx := 0
	for reader.Scan() {
		line := reader.Text()

		idx++
		if idx%1000 == 0 {
			commControl.Mutex.Lock()
			if commControl.Closed {
				commControl.Mutex.Unlock()
				break
			}
			commControl.Mutex.Unlock()
			//fmt.Println(line)
		}

		if !strings.Contains(line, "ACPU PVS:") {
			continue
		}

		logline := cpuprof.ParseLogline(line)
		if logline == nil {
			continue
		}
		pvsStr := logline.Payload[len(logline.Payload)-1:]
		pvsInt, err := strconv.Atoi(pvsStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%s - %s: Failed to convert to int: %v: %v", deviceId, file, logline.Payload, err))
			return
		}
		commControl.Mutex.Lock()
		if commControl.Closed {
			// Nothing to do..already found
			commControl.Mutex.Unlock()
			break
		}
		pvs := &Pvs{}
		pvs.DeviceId = deviceId
		pvs.PvsBin = pvsInt
		pvsChannel <- pvs
		close(commControl.Channel)
		commControl.Closed = true
		logger.Infoln(fmt.Sprintf("Found PVS! %s-%v", deviceId, pvsStr))
		commControl.Mutex.Unlock()
	}
	done++
	logger.Infoln(fmt.Sprintf("DONE: %s - %s (%d)", deviceId, file, done))
}

func PvsMain(args []string) {
	kingpin.MustParse(kpin.Parse(args[1:]))
	files, err := gocommons.ListFiles(*path, []string{"*.out.gz"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not list files")
		os.Exit(-1)
	}

	deviceDataMap := make(map[string]*DeviceData)
	pvsMap := make(map[string]int)

	pvsChecker := func(pvsChannel chan *Pvs, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			if pvs, ok := <-pvsChannel; !ok {
				break
			} else {
				pvsMap[pvs.DeviceId] = pvs.PvsBin
			}
		}
	}

	deviceHandler := func(deviceId string, deviceData *DeviceData, wg *sync.WaitGroup) {
		defer wg.Done()
		deviceWg := &sync.WaitGroup{}
		pvsWg := &sync.WaitGroup{}
		logger.Infoln("Starting handler for device:", deviceId)
		pvsWg.Add(1)
		go pvsChecker(deviceData.PvsChannel, pvsWg)
		sem := gsync.NewSem(8)
		for {
			if file, ok := <-deviceData.DeviceFileChannel; !ok {
				break
			} else {
				logger.Infoln(fmt.Sprintf("Starting file processor: %s - %s", deviceId, file))
				sem.P()
				deviceWg.Add(1)
				go func() {
					defer sem.V()
					defer deviceWg.Done()
					started++
					logger.Infoln("Started:", started)
					ProcessPvs(deviceId, file, deviceData.CommControl, deviceData.PvsChannel)
				}()
			}
		}
		deviceWg.Wait()
		logger.Infoln("Finished devices..closing pvs channel")
		close(deviceData.PvsChannel)
		pvsWg.Wait()
		logger.Infoln("Finished handling device:", deviceId)
	}

	wg := new(sync.WaitGroup)
	for _, file := range files {
		deviceId := strings.Split(file, "/")[3]
		if _, ok := deviceDataMap[deviceId]; !ok {
			deviceData := new(DeviceData)
			deviceData.DeviceFileChannel = make(chan string, 100)
			deviceData.CommControl = new(CommControl)
			deviceData.CommControl.Channel = make(chan struct{})
			deviceData.PvsChannel = make(chan *Pvs)
			deviceDataMap[deviceId] = deviceData
			wg.Add(1)
			go deviceHandler(deviceId, deviceData, wg)
		}
		deviceDataMap[deviceId].DeviceFileChannel <- file
	}
	for deviceId := range deviceDataMap {
		close(deviceDataMap[deviceId].DeviceFileChannel)
	}

	wg.Wait()
	fmt.Println(pvsMap)
	if b, err := json.MarshalIndent(pvsMap, "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		ioutil.WriteFile("pvs.json", b, 0664)
	}
}

func main() {
	logger = logrus.New()
	PvsMain(os.Args)
}
