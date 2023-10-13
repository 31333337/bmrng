package utils

import (
	"fmt"
	"log"
	"regexp"
	"runtime"
)

var debugLogEnabled = false
var debugLogCallerEnabled = false

func SetDebugLogEnabled(enabled bool) {
	debugLogEnabled = enabled
}

func SetDebugLogCallerEnabled(enabled bool) {
	debugLogCallerEnabled = enabled
}

func DebugLog(format string, args ...interface{}) {
	if !debugLogEnabled {
		return
	}

	caller := ""
	if debugLogCallerEnabled {
		// get the calling function's name, filename, and line number
		pc, filename, line, _ := runtime.Caller(1)
		funcname := runtime.FuncForPC(pc).Name()

		// remove project root to shorten file path
		re := regexp.MustCompile("(.*)/trellis/")
		path := re.ReplaceAllString(filename, "")

		caller = fmt.Sprintf("%s:%s:%d ", path, funcname, line)
	}

	s := fmt.Sprintf(format, args...)
	log.Printf("🔷 %s%s", caller, s)
}

// Time statistics tracker
type TimeStats struct {
	Avg   float64
	Last  float64
	Max   float64
	Min   float64
	Total float64

	times []float64
}

func NewTimeStats() *TimeStats {
	return &TimeStats{}
}

func (ts *TimeStats) RecordTime(time float64) {
	init := len(ts.times) == 0

	if init || time < ts.Min {
		ts.Min = time
	}

	if init || time > ts.Max {
		ts.Max = time
	}

	ts.Last = time

	ts.Total += time

	ts.times = append(ts.times, time)

	ts.Avg = ts.Total / float64(len(ts.times))
}

func (ts *TimeStats) GetStatsString() string {
	var ms float64 = 1000000
	return fmt.Sprintf("last/min/avg/max = %.2f / %.2f / %.2f / %.2f ms", ts.Last/ms, ts.Min/ms, ts.Avg/ms, ts.Max/ms)
}
