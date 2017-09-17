package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"syscall"
)

func webhookHandler(w http.ResponseWriter, req *http.Request) {
	currentStatus.WebhookRunning = true
	defer func() {
		currentStatus.WebhookRunning = false
	}()

	tmpfile, err := ioutil.TempFile("", "params")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tempfile err: %v\n", err)
		return
	}
	defer os.Remove(tmpfile.Name())
	if req.Method == "POST" {
		io.Copy(tmpfile, req.Body)
	}
	tmpfile.Close()

	cmd := exec.Command("/bin/sh", strFlag("webhook-script", ""), tmpfile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook start err: %v\n", err)
		return
	}
	err = cmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook err: %v\n", err)
		return
	}
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FindProcess err: %v\n", err)
		return
	}
	err = p.Signal(syscall.SIGUSR1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Signal err: %v\n", err)
	}
}
