package customLogger

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	ERROR   = "Error"
	INFO    = "Info"
	WARNING = "Warning"
	DEBUG   = "Debug"
)

type LogJSON struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

func Stdout(interfaces ...interface{}) {
	for _, i := range interfaces {
		someObject, _ := json.Marshal(i)
		fmt.Fprintln(os.Stdout, string(someObject))
	}
}

func Stderr(interfaces ...interface{}) {
	for _, i := range interfaces {
		someObject, _ := json.Marshal(i)
		fmt.Fprintln(os.Stderr, string(someObject))
	}
}

func StdoutLog(s1, s2, s3 string) {
	Stdout(LogJSON{
		Status:      s1,
		Message:     s2,
		Description: s3,
	})
}

func StderrLog(s1, s2, s3 string) {
	Stderr(LogJSON{
		Status:      s1,
		Message:     s2,
		Description: s3,
	})
}
