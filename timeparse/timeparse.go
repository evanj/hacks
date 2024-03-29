package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"time"
)

// this is 2038-01-19T03:14:07Z
const time32BitLimit = (1 << 32) - 1

// reasonable time range is used to guess at the time format: we guess times in this range
var reasonableStartTime = time.Date(1990, time.January, 1, 2, 3, 4, 0, time.UTC)
var reasonableEndTime = time.Unix(time32BitLimit, 0)

func formatTime(t time.Time) string {
	return fmt.Sprintf("  LOCAL: %s  UTC: %s  UNIX EPOCH: %d",
		t.Local().Format(time.RFC3339Nano), t.UTC().Format(time.RFC3339Nano), t.Unix())
}

func printTime(t time.Time) {
	fmt.Println(formatTime(t))
}

type timeParseFunc func(t string) (time.Time, error)

type timeFormat struct {
	name   string
	parser timeParseFunc
}

func parseEpochSeconds(t string) (time.Time, error) {
	intVal, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(intVal, 0), nil
}

func parseEpochMillis(t string) (time.Time, error) {
	intVal, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, intVal*1000*1000), nil
}

func parseEpochMicros(t string) (time.Time, error) {
	intVal, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, intVal*1000), nil
}

func parseEpochNanos(t string) (time.Time, error) {
	intVal, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, intVal), nil
}

// Parses t as the YY-MM-DDTHH:MM:SS format without a timezone. It assumes UTC.
func parseRFC3339AsUTC(t string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, t+"Z")
}

// makeParseFormat returns a timeParseFunc for a time.Parse layout.
func makeParseFormat(layout string) timeParseFunc {
	return func(value string) (time.Time, error) {
		return time.Parse(layout, value)
	}
}

// time layout for the date/time displayed on Datadog dashboards
const datadogLayout = "Jan 2, 2006, 3:04 pm"

var formats = []timeFormat{
	{"epoch_ns", parseEpochNanos},
	{"epoch_ms", parseEpochMillis},
	{"epoch_us", parseEpochMicros},
	{"epoch_s", parseEpochSeconds},
	{"rfc3339", makeParseFormat(time.RFC3339Nano)},
	{"rfc3339_no_tz", parseRFC3339AsUTC},
	{"rfc1123", makeParseFormat(time.RFC1123)},
	{"unix_date", makeParseFormat(time.UnixDate)},
	// TODO: datadog times may omit years, in which case they should use the "current" year
	// TODO: datadog times could be resolved as "local" or "utc" but default to local
	{"datadog", makeParseFormat(datadogLayout)},
}

func tryParse(input string) (string, time.Time, error) {
	for _, format := range formats {
		t, err := format.parser(input)
		if err != nil {
			continue
		}

		// the parsed time is non-sensical: skip it
		if !(reasonableStartTime.Before(t) && t.Before(reasonableEndTime)) {
			continue
		}

		return format.name, t, nil
	}

	return "", time.Time{}, errors.New("unknown time format: " + input)
}

func main() {
	rangeFlag := flag.Duration("range", 0, "time in the past to print. Units = ns us ms s m h")
	flag.Parse()

	if flag.NArg() == 0 {
		t := time.Now()
		printTime(t)

		if *rangeFlag != 0 {
			t = t.Add(-*rangeFlag)
			fmt.Println()
			printTime(t)
		}
		return
	}

	for _, arg := range flag.Args() {
		name, value, err := tryParse(arg)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		fmt.Printf("%s (%s)\n", arg, name)
		printTime(value)
	}
}
