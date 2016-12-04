package post_processing

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gurupras/cpuprof/post_processing/filters"
	"github.com/gurupras/gocommons"
)

type Boot struct {
	Path       string
	DeviceId   string
	BootId     string
	Files      []string
	CurrentIdx int
	ReadLock   sync.Mutex
}

func NewBoot(path, deviceid, bootid string) *Boot {
	b := Boot{}
	b.Path = path
	b.DeviceId = deviceid
	b.BootId = bootid

	fpath := b.GetBootPath()
	if files, err := gocommons.ListFiles(fpath, []string{"*.gz"}); err != nil {
		os.Exit(-1)
	} else {
		b.Files = files
	}
	b.CurrentIdx = -1
	return &b
}

func (b *Boot) GetBootPath() string {
	return filepath.Join(b.Path, b.DeviceId, b.BootId)
}

/* Expected to be executed in a go-routine */
func (b *Boot) AsyncFilterRead(channel chan string, filters []filters.LineFilter) {
	b.ReadLock.Lock()
	var file_raw *gocommons.File
	var reader *bufio.Scanner
	var err error

	// Initialization
	b.CurrentIdx = 0
	for b.CurrentIdx = 0; b.CurrentIdx < len(b.Files); b.CurrentIdx++ {
		file := b.Files[b.CurrentIdx]
		//fmt.Println("Reading file:", file)
		if file_raw, err = gocommons.Open(file, os.O_RDONLY, gocommons.GZ_TRUE); err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to open file:", err))
			os.Exit(-1)
		}
		if reader, err = file_raw.Reader(1048576); err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to read file:", err))
			os.Exit(-1)
		}
		reader.Split(bufio.ScanLines)
		for reader.Scan() {
			line := reader.Text()
			// Filter
			pass := true
			if len(filters) > 0 {
				for _, filter := range filters {
					pass = pass && filter(line)
					if pass == false {
						break
					}
				}
			}
			if pass {
				channel <- line
			}
		}
		file_raw.Close()
	}
	// Signal done
	close(channel)
	b.CurrentIdx = -1
	b.ReadLock.Unlock()
}

func (b *Boot) AsyncRead(channel chan string) {
	filters := make([]filters.LineFilter, 0)
	b.AsyncFilterRead(channel, filters)
}
