package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

func main() {
	fmt.Printf("runtime.GOARCH=%s\n", runtime.GOARCH)
	fmt.Printf("runtime.Compiler=%s\n\n", runtime.Compiler)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("debug.ReadBuildInfo() failed")
	}
	fmt.Printf("buildinfo.GoVersion=%s (version of the Go toolchain that built this command)\n",
		buildInfo.GoVersion)
	fmt.Println("buildinfo.Settings:")
	for _, setting := range buildInfo.Settings {
		fmt.Printf("  %s=%s\n", setting.Key, setting.Value)
	}
	fmt.Println()

	err := osSpecificOutput()
	if err != nil {
		panic(err)
	}
}
