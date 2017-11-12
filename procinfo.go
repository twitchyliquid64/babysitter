package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type procInfo struct {
	PSS          uint64
	ScheduleTime time.Duration
}

func readIntAdditive(f io.Reader, prefix []byte) (uint64, error) {
	var res uint64
	r := bufio.NewScanner(f)
	for r.Scan() {
		line := r.Bytes()
		if bytes.HasPrefix(line, prefix) {
			var size uint64
			_, err := fmt.Sscanf(string(line[len(prefix):]), "%d", &size)
			if err != nil {
				return 0, err
			}
			res += size
		}
	}
	if err := r.Err(); err != nil {
		return 0, err
	}
	return res, nil
}

func getInfoForProcess(pid int) (procInfo, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/smaps", pid))
	if err != nil {
		return procInfo{}, err
	}
	defer f.Close()

	pss, err := readIntAdditive(f, []byte("Pss:"))
	if err != nil {
		return procInfo{}, err
	}

	statData, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procInfo{}, err
	}
	statSpl := strings.Split(string(statData), " ")
	usrClockTicks, _ := strconv.ParseUint(statSpl[13], 10, 64)

	return procInfo{
		PSS:          pss * 1024,
		ScheduleTime: time.Duration(usrClockTicks) * time.Second / 100, // 100hz
	}, nil
}
