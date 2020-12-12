package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// getDiscard does an HTTP GET and returns the number of bytes in the body.
func getDiscard(pingURL string) (int64, error) {
	resp, err := http.Get(pingURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.New("wrong HTTP status: " + resp.Status)
	}
	n, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		return 0, err
	}
	return n, resp.Body.Close()
}

func main() {
	slowThreshold := flag.Duration("slowThreshold", 100*time.Millisecond,
		"Print if request takes longer than this")
	minInterval := flag.Duration("minInterval", time.Millisecond,
		"Minimum interval between checks")
	reportInterval := flag.Duration("reportInterval", 15*time.Second,
		"Interval to report some statistics")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: httpping (url)")
		os.Exit(1)
	}
	pingURL := flag.Arg(0)

	log.Printf("pinging %s ...", pingURL)

	reportStart := time.Now()
	nextReport := reportStart.Add(*reportInterval)
	count := 0
	var totalBodyBytes int64
	maxDuration := time.Duration(0)
	for {
		start := time.Now()
		bodyBytes, err := getDiscard(pingURL)
		if err != nil {
			panic(err)
		}
		end := time.Now()
		totalBodyBytes += bodyBytes

		duration := end.Sub(start)
		count++
		if duration > maxDuration {
			maxDuration = duration
		}

		if duration > *slowThreshold {
			log.Printf("slow request duration=%s; start=%s; end=%s",
				duration.String(), start.String(), end.String())
		}

		if end.After(nextReport) {
			reportDuration := end.Sub(reportStart)
			log.Printf("%d requests in %s = %.2f req/sec rate; slowest=%s ; total %d body bytes = %.1f bytes/req",
				count, reportDuration.String(), float64(count)/reportDuration.Seconds(), maxDuration,
				totalBodyBytes, float64(totalBodyBytes)/float64(count))
			count = 0
			maxDuration = 0
			totalBodyBytes = 0
			reportStart = end
			nextReport = end.Add(*reportInterval)
		}

		diff := *minInterval - duration
		if diff > 0 {
			time.Sleep(diff)
		}
	}
}
