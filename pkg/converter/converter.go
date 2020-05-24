package converter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func ParseClickHouseTable(pathFile string) (resultObject map[string]string, CHLocalStructure string, err error) {
	resultObject = make(map[string]string)
	file, err := os.Open(pathFile)
	if err != nil {
		return resultObject, "", err
	}
	defer file.Close()

	var columns = map[string]string{}
	var scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		arr := strings.Split(scanner.Text(), "\t")
		resultObject[arr[0]] = arr[1]
		columns[arr[0]] = arr[0] + " " + arr[1] + " " + arr[2] + " " + arr[3]
	}

	keys := make([]string, 0, len(columns))
	for k := range columns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for idx, k := range keys {
		CHLocalStructure += columns[k]
		if idx != len(keys)-1 {
			CHLocalStructure += ", "
		}
	}

	if err := scanner.Err(); err != nil {
		return resultObject, CHLocalStructure, err
	}

	return resultObject, CHLocalStructure, nil
}

func GetParsedMessage(b []byte, columnsMap map[string]string) (string, error) {
	var (
		result string
		obj    map[string]interface{}
		keys   []string
		err    error
	)

	err = json.Unmarshal(b, &obj)
	if err != nil {
		return "", err
	}
	for key, _ := range columnsMap {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for idx, key := range keys {
		switch obj[key].(type) {
		case float64:
			obj[key] = strconv.FormatFloat(obj[key].(float64), 'f', -1, 64)
		case nil:
			obj[key] = nil
		}
		if obj[key] != nil {
			result += "\"" + fmt.Sprintf("%v", obj[key]) + "\""
		} else {
			result += ""
		}
		if idx != len(keys)-1 {
			result += ","
		}
	}
	return result, nil
}
