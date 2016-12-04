package cpuprof

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInfo(t *testing.T) {
	assert := assert.New(t)
	var err error
	var ret map[string][]string
	var bootIds []string
	var path string

	// Test with invalid file
	path = "./test_files/common/GetInfo/non-existant-directory"
	ret, err = GetInfo(path)
	assert.Nil(ret, "Should have received nil map")
	assert.NotNil(err, "Should have failed with non-existant path")

	// Test with correct, but different JSON
	path = "./test_files/common/GetInfo/test-random-valid-json"
	ret, err = GetInfo(path)
	assert.NotNil(ret, "Should have succeeded")
	assert.NotNil(err, "Should have Failed")
	bootIds, err = GetBootIds(path)
	assert.Nil(bootIds, "Got bootids from invalid json")
	assert.NotNil(err, "Got bootids from invalid json")

	// Test with correct JSON
	path = "./test_files/common/GetInfo/test-correct-json"
	ret, err = GetInfo(path)
	assert.NotNil(ret, "Should have succeeded")
	assert.Nil(err, "Should have succeeded")
	bootIds, err = GetBootIds(path)
	assert.NotNil(bootIds, "Got not bootids from valid json")
	assert.Nil(err, "Got error from valid json")
}

func TestGetBootIds(t *testing.T) {
	assert := assert.New(t)

	// Test with correct JSON
	var err error
	path := "./test_files/common/GetInfo/test-correct-json"
	bootIds, err := GetBootIds(path)
	fmt.Println(bootIds)
	assert.True(len(bootIds) > 0, "Got not bootids from valid json")
	assert.Nil(err, "Got error from valid json")
}
