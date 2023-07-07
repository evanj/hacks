package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var flagsPattern = regexp.MustCompile(`^flags\s*:\s*(.*)$`)

func readCPUFlags() ([]string, error) {
	cpuinfoF, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	defer cpuinfoF.Close()

	scanner := bufio.NewScanner(cpuinfoF)
	for scanner.Scan() {
		matches := flagsPattern.FindSubmatch(scanner.Bytes())
		if len(matches) == 0 {
			continue
		}

		return strings.Split(string(matches[1]), " "), nil
	}

	return nil, nil
}

func readCPULevel() (int, error) {
	flags, err := readCPUFlags()
	if err != nil {
		return 0, err
	}

	flagsSet := map[string]struct{}{}
	for _, flag := range flags {
		flagsSet[flag] = struct{}{}
	}

	// RHEL9 targets x86-64-v2, which is approximately
	//
	// https://developers.redhat.com/blog/2021/01/05/building-red-hat-enterprise-linux-9-for-the-x86-64-v2-microarchitecture-level#background_of_the_x86_64_microarchitecture_levels
	// https://unix.stackexchange.com/questions/631217/how-do-i-check-if-my-cpu-supports-x86-64-v2
	//
	// See:
	// https://en.wikipedia.org/wiki/X86-64#Microarchitecture_levels
	//
	// Official definition section in the x86-64 ABI section 3.1.1 Processor Architecture:
	// https://gitlab.com/x86-psABIs/x86-64-ABI
	//
	// v2 is approximately Intel Nehalem (2008) / AMD Bulldozer (2011)
	// v3 is approximately Intel Haswell (2013)
	levels := [][]string{
		{"lm", "cmov", "cx8", "fpu", "fxsr", "mmx", "syscall", "sse2"},
		{"cx16", "lahf_lm", "popcnt", "sse4_1", "sse4_2", "ssse3"},
		{"avx", "avx2", "bmi1", "bmi2", "f16c", "fma", "abm", "movbe", "xsave"},
		{"avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl"},
	}

	for level, levelFlags := range levels {
		for _, flag := range levelFlags {
			if _, ok := flagsSet[flag]; !ok {
				fmt.Printf("missing cpu flag %#v for level %d; CPU supports level %d\n",
					flag, level+1, level)
				return level, nil
			}
		}
	}
	return len(levels), nil
}

func osSpecificOutput() error {
	level, err := readCPULevel()
	if err != nil {
		return err
	}
	fmt.Printf("CPU supports level %d (x86-64-v%d)\n", level, level)
	return nil
}
