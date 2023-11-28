package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/evanj/hacks/nilslog"
	"github.com/evanj/hacks/postgrestest"
	"golang.org/x/exp/slog"
)

const psqlBinName = "psql"

func startPostgresAndPSQL(listenOnLocalhost bool, verbose bool, insecureGlobalPort int, dir string) {
	logger := nilslog.New()
	if verbose {
		logger = slog.Default()
	}

	options := postgrestest.Options{
		ListenOnLocalhost:  listenOnLocalhost,
		Logger:             logger,
		InsecureGlobalPort: insecureGlobalPort,
		DirPath:            dir,
	}
	instance, err := postgrestest.NewInstanceWithOptions(options)
	if err != nil {
		panic(err)
	}
	defer instance.Close()

	if insecureGlobalPort != 0 {
		fmt.Printf("remote URL: %s\n", instance.RemoteURL())
	}

	fmt.Printf("starting psql connection=%s ...\n", instance.URL())
	psql := exec.Command(instance.BinPath(psqlBinName), instance.URL())
	psql.Stdin = os.Stdin
	psql.Stdout = os.Stdout
	psql.Stderr = os.Stderr

	// register a sigint handler
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigintChan)

	psqlResult := make(chan error, 1)
	go func() {
		options.Logger.Info("running psql", "cmd_line", strings.Join(psql.Args, " "))
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

func main() {
	listenOnLocalhost := flag.Bool("listenOnLocalhost", false, "Listens on localhost if set")
	insecureGlobalPort := flag.Int("insecureGlobalPort", 0, "If set, listens for global TCP connections")
	verbose := flag.Bool("verbose", false, "Logs verbose commands if set")
	dir := flag.String("dir", "", "If not empty, use and/or create DB in this dir (for reusing directory)")
	flag.Parse()

	startPostgresAndPSQL(*listenOnLocalhost, *verbose, *insecureGlobalPort, *dir)
}
