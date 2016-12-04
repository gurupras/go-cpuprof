package cpuprof

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func GetInfo(path string) (json_map map[string][]string, err error) {
	var fpath string
	var bytes []byte

	fpath = filepath.Join(path, "info.json")
	if bytes, err = ioutil.ReadFile(fpath); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read file:", fpath, ":", err)
		return
	}
	if err = json.Unmarshal(bytes, &json_map); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to unmarshal bytes:", err)
		return
	}
	return
}

func GetBootIds(path string) (bootids []string, err error) {
	json_map, err := GetInfo(path)
	bootids = json_map["bootids"]
	return
}
