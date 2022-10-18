package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/evanj/hacks/postgrestest"
)

func main() {
	listenOnLocalhost := flag.Bool("listenOnLocalhost", false, "Listens on localhost if set")
	flag.Parse()

	instance, err := postgrestest.NewInstanceWithOptions(postgrestest.Options{ListenOnLocalhost: *listenOnLocalhost})
	if err != nil {
		panic(err)
	}
	defer instance.Close()

	fmt.Printf("starting psql connection=%s ...\n", instance.URL())
	psql := exec.Command("psql", instance.URL())
	psql.Stdin = os.Stdin
	psql.Stdout = os.Stdout

	// register a sigint handler
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigintChan)

	psqlResult := make(chan error, 1)
	go func() {
		psqlResult <- psql.Run()
	}()
	select {
	case err := <-psqlResult:
		if err != nil {
			panic(fmt.Sprintf("psql exited=%s", err.Error()))
		}

	case sig := <-sigintChan:
		fmt.Printf("postgrestmp handling signal=%s\n", sig.String())
		return
	}
}
