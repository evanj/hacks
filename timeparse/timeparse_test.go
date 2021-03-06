package main

import (
	"testing"
	"time"
)

func TestTryParse(t *testing.T) {
	nycTZ, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		input string
		name  string
		value time.Time
	}
	testCases := []testCase{
		{"2020-04-06T17:16:39.774342", "rfc3339_no_tz",
			time.Date(2020, 4, 6, 17, 16, 39, 774342000, time.UTC)},
		{"1591641566", "epoch_s",
			time.Date(2020, 6, 8, 18, 39, 26, 0, time.UTC)},
		{"Wed, 10 Jun 2020 20:01:31 GMT", "rfc1123",
			time.Date(2020, 6, 10, 20, 1, 31, 0, time.UTC)},
		{"Sat Dec 12 13:27:44 EST 2020", "unix_date",
			time.Date(2020, 12, 12, 13, 27, 44, 0, nycTZ)},
		{"Dec 29, 2020, 5:03 am", "datadog",
			time.Date(2020, 12, 29, 5, 3, 0, 0, time.UTC)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			name, value, err := tryParse(testCase.input)
			if err != nil {
				t.Errorf("tryParse(%#v) returned unexpected error %s", testCase.input, err.Error())
				return
			}

			if name != testCase.name {
				t.Errorf("tryParse(%#v) returned name=%s; expected %s",
					testCase.input, name, testCase.name)
			}
			value = value.UTC()
			if !value.Equal(testCase.value) {
				t.Errorf("tryParse(%#v) returned value=%s; expected %s",
					testCase.input, value, testCase.value)
			}
		})
	}
}
