package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/shlex"
	"github.com/gurupras/cpuprof"
	"github.com/gurupras/gocommons"
	"github.com/stretchr/testify/assert"
)

func TestStitch(t *testing.T) {
	var success bool = false
	var err error

	result := gocommons.InitResult("TestStitch")
	//cmdline, err := shlex.Split("stitch_test.go /android/test-cpuprof/1b0676e5fb2d7ab82a2b76887c53e94cf0410826 --regex *.out.gz")
	cmdline, err := shlex.Split("stitch_test.go /android/cpuprof-data/1a28ea49f4206010fee054f9bdb86f822dc4dd28")
	StitchMain(cmdline)
	if err == nil {
		success = true
	}

	gocommons.HandleResult(t, success, result)
}

func TestWriteBootIdsJson(t *testing.T) {
	var success bool = true
	var err error

	result := gocommons.InitResult("TestWriteBootIdsJson")

	path := "."
	file := filepath.Join(path, "bootids.json")
	bootids := []string{"a", "b", "c", "d"}
	if err = WriteBootIdsJson(path, bootids); err != nil {
		success = false
	}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to read file:", err))
		success = false
	}
	var check []string
	if err := json.Unmarshal(bytes, &check); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to unmarshal file:", err))
		success = false
	}

	for i := range bootids {
		if bootids[i] != check[i] {
			success = false
			break
		}
	}
	os.Remove(file)
	gocommons.HandleResult(t, success, result)
}

func TestIncreasingLogcatToken(t *testing.T) {
	path := "/android/cpuprof-data/1a28ea49f4206010fee054f9bdb86f822dc4dd28/e3ee246f-1970-4d78-ac04-483491206468"
	files, _ := gocommons.ListFiles(path, []string{"*.gz"})
	sort.Sort(sort.StringSlice(files))

	testIncreasingLogcatToken(t, files)
}

func testIncreasingLogcatToken(t *testing.T, files []string) {
	var infile_raw *gocommons.File
	var err error
	var reader *bufio.Scanner

	current_token := int64(0)
	for _, file_path := range files {
		fmt.Println("Testing:", file_path)
		if infile_raw, err = gocommons.Open(file_path, os.O_RDONLY, gocommons.GZ_FALSE); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to open:", file_path, ":", err)
			return
		}
		defer infile_raw.Close()
		if reader, err = infile_raw.Reader(0); err != nil {
			fmt.Fprintln(os.Stderr, "Could not get reader:", file_path)
			return
		}

		reader.Split(bufio.ScanLines)
		for reader.Scan() {
			line := reader.Text()
			logline := cpuprof.ParseLogline(line)
			if logline != nil {
				gt := current_token < logline.LogcatToken
				assert.Equal(t, true, gt, "Logcat tokens not sorted properly")
				current_token = logline.LogcatToken
			}
		}
	}
}
