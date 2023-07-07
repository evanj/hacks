package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func osSpecificOutput() error {
	const darwinBrandString = "machdep.cpu.brand_string"
	cpuName, err := unix.Sysctl(darwinBrandString)
	if err != nil {
		return err
	}
	fmt.Printf("sysctl %#v=%#v\n", darwinBrandString, cpuName)
	fmt.Printf("TODO: implement CPU architecture level detection for darwin\n")
	return nil
}
