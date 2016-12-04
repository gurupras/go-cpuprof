package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/set"
	"github.com/gurupras/cpuprof"
	"github.com/gurupras/gocommons"
)

var (
	kpin       = kingpin.New("stitch", "")
	path       = kpin.Arg("path", "").Required().String()
	regex      = kpin.Flag("regex", "").Short('r').Default("*.out.gz").String()
	bufsize    = kpin.Flag("bufsize", "Buffer size per thread").Short('b').Default("104857600").Int()
	split_only = kpin.Flag("split-only", "Only perform split with existing chunks").Short('s').Default("false").Bool()
	delete     = kpin.Flag("delete", "Delete intermediate files on exit").Default("false").Bool()
)

func setAddStrings(s set.Interface, items []string) {
	for _, item := range items {
		s.Add(item)
	}
}

func Process(path string, regex string, bufsize int) error {
	var files []string
	var overall_chunks []string
	var err error
	chunk_chan := make(chan []string)
	var merged_files []string
	var old_bootids *set.SetNonTS
	var merged_bootids []string
	var info map[string][]string
	delete_boot_ids := true

	// Split regexes by ','
	patterns := strings.Split(regex, ",")
	if files, err = gocommons.ListFiles(path, patterns); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to list files:", path, ":", err))
		os.Exit(-1)
	}
	if info, err = cpuprof.GetInfo(path); err != nil {
		fmt.Println("Did not find info file...Using all files")
		merged_files = files
		old_bootids = set.NewNonTS()
	} else {
		fmt.Println("Found info.json...Finding new files to process")
		old_files := set.NewNonTS()
		setAddStrings(old_files, info["files"])

		files_set := set.NewNonTS()
		setAddStrings(files_set, files)

		diff_set := set.Difference(files_set, old_files)
		files = set.StringSlice(diff_set)
		merged_files = set.StringSlice(set.Union(old_files, files_set))

		old_bootids = set.NewNonTS()
		setAddStrings(old_bootids, info["bootids"])
		fmt.Println("New files:", files)
		delete_boot_ids = false
	}

	ext_sort := func(file string) {
		var chunks []string
		if chunks, err = gocommons.ExternalSort(file, bufsize, cpuprof.LoglineSortParams); err != nil {
			return
		}
		chunk_chan <- chunks
	}

	fmt.Println("Starting external sort")
	for _, file := range files {
		go ext_sort(file)
	}
	// Wait for them to complete
	for i := 0; i < len(files); i++ {
		chunk := <-chunk_chan
		overall_chunks = append(overall_chunks, chunk...)
	}

	sort.Sort(sort.StringSlice(overall_chunks))
	// Split by boot-id
	bootids := BootIdSplit(path, files, overall_chunks, delete_boot_ids, 1000000)

	// Delete intermediate files if requested
	if *delete == true {
		for _, chunk := range overall_chunks {
			if err = os.Remove(chunk); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to remove chunk:", chunk)
			}
		}
	}

	new_bootids := set.NewNonTS()
	setAddStrings(new_bootids, bootids)
	merged_bootids = set.StringSlice(set.Union(old_bootids, new_bootids))
	// Now write the json stating the various bootids and files processed
	WriteInfoJson(path, merged_files, merged_bootids)
	return err
}

func BootIdSplit(path string, files []string, chunks []string, delete bool, lines_per_file int) (bootids []string) {
	merge_out_channel := make(chan gocommons.SortInterface, 10000)

	var err error

	bootid_channel_map := make(map[string]chan string)

	callback := func(out_channel chan gocommons.SortInterface, quit chan bool) {
		boot_id_consumer := func(boot_id string, channel chan string, wg *sync.WaitGroup) {
			defer wg.Done()
			var err error

			//fmt.Println("Starting consumer for bootid:", boot_id)
			cur_idx := 0
			cur_line_count := 0
			outdir := filepath.Join(path, boot_id)
			var cur_filename string
			var cur_file *gocommons.File
			var cur_file_writer gocommons.Writer

			// Make directory if it doesn't exist
			if delete {
				if _, err := os.Stat(outdir); os.IsExist(err) {
					// Does exit
					fmt.Println("Attempting to delete existing directory:", outdir)
					os.RemoveAll(outdir)
				}
			}
			fmt.Printf("Attempting to create directory:%s...", outdir)
			if err = os.MkdirAll(outdir, 0775); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to create directory:", outdir)
				os.Exit(-1)
			} else {
				fmt.Printf("Done\n")
			}
			if !delete {
				var files []string

				if files, err = gocommons.ListFiles(outdir, []string{"*.gz"}); err != nil {
					fmt.Fprintln(os.Stderr, "Failed to list files:", outdir)
					os.Exit(-1)
				}
				sort.Sort(sort.StringSlice(files))
				if len(files) > 0 {
					last_file := files[len(files)-1]
					if cur_idx, err = strconv.Atoi(strings.TrimSuffix(last_file, filepath.Ext(last_file))); err != nil {

					}
					// Move it ahead by 1
					cur_idx++
				}
			}

			first_file := true
			for {
				if cur_line_count == 0 {
					// Line count is 0. Either this is the first file or we just reset stuff
					// and so we need to open a new file
					if !first_file {
						// Close the old file
						//fmt.Println("\tClosing old file")
						cur_file_writer.Flush()
						cur_file_writer.Close()
						cur_file.Close()
					} else {
						first_file = false
					}
					// Open up a new file
					cur_filename = filepath.Join(outdir, fmt.Sprintf("%08d.gz", cur_idx))
					//fmt.Println("New File:", cur_filename)
					if cur_file, err = gocommons.Open(cur_filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, gocommons.GZ_TRUE); err != nil {
						fmt.Fprintln(os.Stderr, "Could not open:", cur_filename, " :", err)
						return
					}
					if cur_file_writer, err = cur_file.Writer(0); err != nil {
						fmt.Fprintln(os.Stderr, "Could not get writer:", cur_filename, " :", err)
						return
					}
				}
				if cur_line_count == lines_per_file {
					// We've reached the allotted lines per file. Rotate.
					//fmt.Println("Rotating to new file ...")
					cur_idx++
					cur_line_count = 0
					// We haven't read a line yet. So we can re-enter the loop here
					continue
				}
				// All the file stuff has been set up.
				// Go ahead and read a line from the channel
				if line, ok := <-channel; !ok {
					// Channel was closed. We're finished reading.
					// Cleanup
					fmt.Println("Cleaning up:", boot_id)
					cur_file_writer.Flush()
					cur_file_writer.Close()
					cur_file.Close()
					break
				} else {
					cur_line_count++
					if cur_line_count != lines_per_file {
						cur_file_writer.Write([]byte(line + "\n"))
					} else {
						cur_file_writer.Write([]byte(line))
					}
				}
			}
		}

		var wg sync.WaitGroup
		for {
			si, ok := <-merge_out_channel
			if !ok {
				goto done
			}
			logline, ok := si.(*cpuprof.Logline)
			if !ok {
				fmt.Fprintln(os.Stderr, "Could not convert to logline:", err)
				os.Exit(-1)
			}

			boot_id := logline.BootId
			// Check if the map has this bootid.
			if _, ok := bootid_channel_map[boot_id]; !ok {
				// Does not. Add it in and create a consumer
				bootids = append(bootids, boot_id)
				bootid_channel_map[boot_id] = make(chan string, 10000)
				wg.Add(1)
				go boot_id_consumer(boot_id, bootid_channel_map[boot_id], &wg)
			} else {
				// Write line to channel
				bootid_channel_map[boot_id] <- logline.Line
			}
		}
	done:
		fmt.Println("Cleaning up callback")
		// Done reading the file. Now close the channels
		for boot_id := range bootid_channel_map {
			close(bootid_channel_map[boot_id])
		}
		fmt.Printf("Waiting for bootid_consumers to complete...")
		wg.Wait()
		fmt.Printf("Done\n")
		quit <- true
		fmt.Println("Callback: Done")
	}
	// Now start the n-way merge generator
	gocommons.NWayMergeGenerator(chunks, cpuprof.LoglineSortParams, merge_out_channel, callback)

	fmt.Println("Bootids:", bootids)
	return bootids
}

func WriteInfoJson(path string, files []string, bootids []string) (err error) {
	var bootid_file_raw *gocommons.File
	var bootid_writer gocommons.Writer

	if bootid_file_raw, err = gocommons.Open(filepath.Join(path, "info.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, gocommons.GZ_FALSE); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to open info.json:", err))
		return
	}
	defer bootid_file_raw.Close()
	if bootid_writer, err = bootid_file_raw.Writer(0); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to open writer to info.json:", err))
		return err
	}
	defer bootid_writer.Close()
	defer bootid_writer.Flush()

	json_map := make(map[string][]string)
	json_map["bootids"] = bootids
	json_map["files"] = files
	var json_string []byte
	if json_string, err = json.MarshalIndent(json_map, "", "    "); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Failed to marshal:", err))
		return err
	}
	bootid_writer.Write(json_string)
	return
}

func StitchMain(args []string) {
	kingpin.MustParse(kpin.Parse(args[1:]))
	if !*split_only {
		Process(*path, *regex, *bufsize)
	} else {
		if files, err := gocommons.ListFiles(*path, []string{"*.out.gz"}); err != nil {
			fmt.Fprintln(os.Stderr, "Could not list files")
			os.Exit(-1)
		} else {
			if chunks, err := gocommons.ListFiles(*path, []string{"*chunk*.gz"}); err != nil {
				fmt.Fprintln(os.Stderr, "Could not list chunks")
				os.Exit(-1)
			} else {
				BootIdSplit(*path, files, chunks, true, 1000000)
			}
		}
	}
}

func main() {
	StitchMain(os.Args)
}
