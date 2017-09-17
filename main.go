package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func run(args []string, wd string, stdout io.Writer, stderr io.Writer) error {
	currentStatus.Running = true
	cmd := exec.Command(args[0], args[1:]...)

	defer func() {
		currentStatus.Running = false
		currentStatus.RestartCount++
		if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(time.Millisecond * 500)
			if !cmd.ProcessState.Exited() {
				cmd.Process.Kill()
			}
		}
	}()

	cmd.Dir = wd
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	err := cmd.Start()
	if err != nil {
		return err
	}
	currentStatus.Pid = cmd.Process.Pid
	finChan := make(chan error, 1)
	go func() {
		finChan <- cmd.Wait()
	}()
	return waitInterrupt(finChan)
}

func outputInit() (io.Writer, io.Writer, error) {
	var stdout, stderr io.Writer
	switch strFlag("stdout", "-") {
	case "-", "stdout":
		stdout = os.Stdout
	case "stderr":
		stdout = os.Stderr
	default:
		f, err := os.OpenFile(strFlag("stdout", "-"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, nil, errors.New("Could not init stdout: " + err.Error())
		}
		stdout = f
	}

	switch strFlag("stderr", "-") {
	case "-", "stderr":
		stderr = os.Stderr
	case "stdout":
		stderr = os.Stdout
	default:
		f, err := os.OpenFile(strFlag("stderr", "-"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, nil, errors.New("Could not init stderr: " + err.Error())
		}
		stderr = f
	}

	return stdout, stderr, nil
}

func main() {
	if flagErr := verifyFlags(); flagErr != nil {
		fmt.Fprintf(os.Stderr, "Invalid options: %v\n", flagErr)
		os.Exit(1)
	}

	outWriter, errWriter, err := outputInit()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	wd, _ := os.Getwd()
	wd = strFlag("dir", wd)

	if flagExists("status-serv") {
		if strFlag("webhook-script", "") != "" && strFlag("webhook-token", "") != "" {
			http.HandleFunc("/webhook/"+strFlag("webhook-token", ""), webhookHandler)
		}
		http.HandleFunc("/", statusPage)
		go func() {
			err := http.ListenAndServe(strFlag("status-serv", ":7000"), nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Listener err: %v\n", err)
			}
		}()
	}

	for {
		runErr := run(extraArgs, wd, outWriter, errWriter)
		tryFlushWriter(outWriter)
		tryFlushWriter(errWriter)
		if runErr == errTerminationSignal {
			fmt.Fprintf(os.Stderr, "Terminating: %v\n", runErr)
			return
		}
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "Run finished with error: %v\n", runErr)
			currentStatus.LastRunError = runErr
		} else {
			currentStatus.LastRunError = nil
		}
		time.Sleep(time.Millisecond * time.Duration(intFlag("restart-delay-ms", 2000)))
	}
}

func tryFlushWriter(outWriter io.Writer) {
	if outWriter == os.Stdout || outWriter == os.Stderr {
		return
	}
	if out, ok := outWriter.(*os.File); ok {
		syncErr := out.Sync()
		if syncErr != nil {
			fmt.Fprintf(os.Stderr, "Sync failed for %s: %v\n", out.Name(), syncErr)
		}
	}
}

var errTerminationSignal = errors.New("recieved termination signal")

func waitInterrupt(fatalErrChan chan error) error {
	sig := make(chan os.Signal, 2)
	done := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer signal.Stop(sig)

	go func() {
		done <- <-sig
	}()

	select {
	case signalKind := <-done:
		if signalKind != syscall.SIGUSR1 {
			return errTerminationSignal
		}
		fmt.Fprintln(os.Stderr, "Recieved SIGUSR1, restarting child")
		return nil
	case err := <-fatalErrChan:
		return err
	}
}
