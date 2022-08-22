package main

import "testing"

func TestIsStandardLibrary(t *testing.T) {
	testCases := []struct {
		packagePath string
		expected    bool
	}{
		{"", false},
		{"strings", true},
		{"net/http", true},
		{"go4.org/unsafe/assume-no-moving-gc", false},
	}

	for i, testCase := range testCases {
		output := isStandardLibrary(testCase.packagePath)
		if output != testCase.expected {
			t.Errorf("%d: isStandardLibrary(%#v)=%t; expected=%t",
				i, testCase.packagePath, output, testCase.expected)
		}
	}
}
