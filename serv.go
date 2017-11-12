package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var startTime time.Time
var processStarted time.Time

func init() {
	startTime = time.Now()
}

type status struct {
	Running      bool
	Pid          int
	RestartCount int
	LastRunError error

	WebhookRunning bool
	WebhookCount   int
}

type statusData struct {
	Listenaddr       string
	Name             string
	Background       string
	Cmd              []string
	ShowFullInfo     bool
	WebhookInstalled bool
	WebhookToken     string
	WebhookScript    string

	Stdout       string
	Stderr       string
	RestartDelay int

	StartTime time.Time
	Status    *status
	Mem       *runtime.MemStats
	DiskFree  uint64
	DiskTotal uint64
	NumCPUs   int

	ProcessPSS          uint64
	ProcessScheduleTime time.Duration
	ProcessErr          error
	ProcessStartedAt    time.Time
}

var currentStatus status

var funcMap = template.FuncMap{
	"bytes": func(bytes uint64) string {
		unit := ""
		value := float32(bytes)

		switch {
		case bytes >= (1024 * 1024 * 1024 * 1):
			unit = "G"
			value = value / (1024 * 1024 * 1024 * 1)
		case bytes >= (1024 * 1024 * 1):
			unit = "M"
			value = value / (1024 * 1024 * 1)
		case bytes >= (1024 * 1):
			unit = "K"
			value = value / (1024 * 1)
		case bytes >= 1:
			unit = "B"
		case bytes == 0:
			return "0"
		}

		return fmt.Sprintf("%s%s", strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0"), unit)
	},
	"percent": func(sub, total uint64) string {
		if total == 0 {
			return "NaN"
		}

		p := (sub * 100) / total
		return strconv.Itoa(int(p))
	},
	"boolcolor": func(in bool) template.HTML {
		if in {
			return template.HTML("<span style=\"color: #00AA00;\">Yes</span>")
		}
		return template.HTML("<span style=\"color: #AA0000;\">No</span>")
	},
	"durationtime": func(in time.Duration) string {
		return fmt.Sprintf("%.2f ms, %d secs, %d mins, %d hrs", float64((in.Nanoseconds()%(1000*1000)))/float64(1000), int(in.Seconds())%60, int(in.Minutes())%60, int(in.Hours()))
	},
	"timeformat": func(in time.Time) string {
		t := time.Now().Sub(in)
		s := strconv.Itoa(int(t.Hours())) + " hours, "
		s += strconv.Itoa(int(t.Minutes())%60) + " minutes, "
		s += strconv.Itoa(int(t.Seconds())%60) + " seconds."
		return s
	},
	"omitIfNotShowFullInfo": func(in string) string {
		if boolFlag("show-full-data", false) {
			return in
		}
		return "<omitted>"
	},
	"renderCommand": func(cmd []string) template.HTML {
		out := "<span>" + template.HTMLEscapeString(cmd[0]) + "</span>"
		if boolFlag("show-full-data", false) {
			for i := 1; i < len(cmd); i++ {
				if strings.HasPrefix(cmd[i], "--") && (i+1) < len(cmd) {
					out += "<br><span class=\"argline\"><span class=\"flag\">" + template.HTMLEscapeString(cmd[i]) + "</span> " + template.HTMLEscapeString(cmd[i+1]) + "</span>"
					i++
				} else {
					if strings.HasPrefix(cmd[i], "-") {
						out += "<br><span class=\"argline\"><span class=\"flag\">" + template.HTMLEscapeString(cmd[i]) + "</span></span>"
					} else {
						out += "<br><span class=\"argline\">" + template.HTMLEscapeString(cmd[i]) + "</span>"
					}
				}
			}
		} else {
			out += "<br><br>(Truncated)"
		}
		return template.HTML(out)
	},
}

func getDiskFreeAndTotal() (uint64, uint64) {
	var stat syscall.Statfs_t
	wd, _ := os.Getwd()
	syscall.Statfs(wd, &stat)
	// Available blocks * size per block = available space in bytes
	return stat.Bavail * uint64(stat.Bsize), stat.Blocks * uint64(stat.Bsize)
}

func statusPage(w http.ResponseWriter, r *http.Request) {
	t, err := template.New("status").Funcs(funcMap).Parse(statusTemplate)
	if err != nil {
		w.Write([]byte("Template Error: " + err.Error()))
		return
	}

	diskFree, diskTotal := getDiskFreeAndTotal()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	processInfo, processErr := getInfoForProcess(currentStatus.Pid)

	err = t.ExecuteTemplate(w, "status", statusData{
		Name:                strFlag("service-name", "Babysitter"),
		Listenaddr:          strFlag("status-serv", ":7000"),
		ShowFullInfo:        boolFlag("show-full-data", false),
		Cmd:                 extraArgs,
		StartTime:           startTime,
		Stdout:              strFlag("stdout", "/dev/stdout"),
		Stderr:              strFlag("stderr", "/dev/stderr"),
		RestartDelay:        intFlag("restart-delay-ms", 2000),
		Background:          strFlag("status-color", "#F0F0F0"),
		WebhookInstalled:    strFlag("webhook-script", "") != "" && strFlag("webhook-token", "") != "",
		WebhookScript:       strFlag("webhook-script", ""),
		WebhookToken:        strFlag("webhook-token", ""),
		Status:              &currentStatus,
		NumCPUs:             runtime.NumCPU(),
		Mem:                 &memStats,
		DiskFree:            diskFree,
		DiskTotal:           diskTotal,
		ProcessErr:          processErr,
		ProcessPSS:          processInfo.PSS,
		ProcessScheduleTime: processInfo.ScheduleTime,
		ProcessStartedAt:    processStarted,
	})
	if err != nil {
		w.Write([]byte("Template Exec Error: " + err.Error()))
	}
}

var statusTemplate = `
<html>
  <head>
    <title>{{.Name}}</title>
  </head>
  <body>
  <style>
    .section-header {
      width: 250px;
    }
    .top {
      font-size: 1.25em;
      font-weight: bold;
    }
  	.main {
      width:100%;
  		border:1px solid #C0C0C0;
  		border-collapse:collapse;
  		padding:5px;
  	}
  	.main th {
  		border:1px solid #C0C0C0;
  		padding:5px;
  		background: {{.Background}};
  	}
  	.main td {
  		border:1px solid #C0C0C0;
  		padding:5px;
  	}
		.flag {
			font-style: italic;
		}
		.argline {
			padding-left: 8px;
		}
  </style>
  <table class="main top">
  	<thead>
    	<tr>
    		<th>{{.Name}}</th>
    	</tr>
  	</thead>
  	<tbody>
		<tr>
			<td>
				<table class="main">
					<thead>
						<tr>
							<th class="section-header">Status</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<td>Running</td>
							<td>{{boolcolor .Status.Running}}</td>
						</tr>
						<tr>
							<td>Last PID</td>
							<td>{{.Status.Pid}}</td>
						</tr>
						<tr>
							<td>Restart count</td>
							<td>{{.Status.RestartCount}}</td>
						</tr>
						<tr>
							<td>Last termination error</td>
							<td>{{.Status.LastRunError}}</td>
						</tr>
						<tr>
							<td>Webhook in progress</td>
							<td>{{boolcolor .Status.WebhookRunning}}</td>
						</tr>
						<tr>
							<td>Webhook invocations</td>
							<td>{{.Status.WebhookCount}}</td>
						</tr>
						{{if .ProcessErr}}
						<tr>
							<td>Resource usage</td>
							<td>Failed to read resource usage: {{.ProcessErr}}</td>
						</tr>
						{{else}}
						<tr>
							<td>Resource usage</td>
							<td>
								Memory usage: {{bytes .ProcessPSS}}<br>
								Uptime: {{timeformat .ProcessStartedAt}}<br>
								CPU time: {{durationtime .ProcessScheduleTime}}
							</td>
						</tr>
						{{end}}
					</tbody>
				</table>
			</td>
		</tr>

    	<tr>
    		<td>
          <table class="main">
            <thead>
              <tr>
                <th class="section-header">Configuration</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
							<tr>
								<td>Show full configuration</td>
								<td>{{boolcolor .ShowFullInfo}}</td>
							</tr>
							<tr>
								<td>Webhook installed</td>
								<td>{{boolcolor .WebhookInstalled}}</td>
							</tr>
              <tr>
                <td>Statusz address</td>
                <td>{{.Listenaddr}}</td>
              </tr>
							{{if .WebhookInstalled -}}
							<tr>
                <td>Webhook script</td>
                <td>{{omitIfNotShowFullInfo .WebhookScript}}</td>
              </tr>
							<tr>
                <td>Webhook token</td>
                <td>{{omitIfNotShowFullInfo .WebhookToken}}</td>
              </tr>
							{{end -}}
							<tr>
                <td>Command</td>
                <td>
									{{renderCommand .Cmd}}
								</td>
              </tr>
							<tr>
                <td>Standard out</td>
                <td>{{.Stdout}}</td>
              </tr>
							<tr>
                <td>Standard error</td>
                <td>{{.Stderr}}</td>
              </tr>
							<tr>
								<td>Restart delay (ms)</td>
								<td>{{.RestartDelay}}</td>
							</tr>
            </tbody>
          </table>
        </td>
    	</tr>

			<tr>
				<td>
					<table class="main">
						<thead>
							<tr>
								<th class="section-header">System</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td>Service uptime</td>
								<td>{{timeformat .StartTime}}</td>
							</tr>
							<tr>
								<td>Num CPUs</td>
								<td>{{.NumCPUs}}</td>
							</tr>
							<tr>
								<td>Disk</td>
								<td>
									Free: {{bytes .DiskFree}} ({{percent .DiskFree .DiskTotal}}%)<br>
									Total: {{bytes .DiskTotal}}
								</td>
							</tr>
							<tr>
								<td>Memory (babysitter)</td>
								<td>
									Allocated: {{bytes .Mem.Sys}}<br>
									GC Count: {{.Mem.NumGC}}
								</td>
							</tr>
						</tbody>
					</table>
				</td>
			</tr>
  	</tbody>
  </table>
  </body>
</html>
`
