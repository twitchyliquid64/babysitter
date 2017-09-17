# babysitter

Babysitter is a simple program to launch daemons, and keep them running.

## Build

```shell
git clone https://github.com/twitchyliquid64/babysitter
cd babysitter
go build -o babysitter
```

## Usage

`babysitter` is configured exclusively through commandline arguments. Commandline arguments are specified with `--flagname <flagvalue>`. The first argument (and all subsequent arguments) which does not match the `--flag value` pattern are invoked as the child daemon.

| Flag          | Explanation |
| --------------| ------------|
| `service-name` | Name to be shown in the web UI if enabled. Defaults to 'babysitter'. |
| `status-serv` | Listener address of the web UI. The web UI & webhooks are disabled if this is not specified. |
| `status-color` | Background color of the web UI. Defaults to #F0F0F0. |
| `show-full-data` | If set to true, the web UI will show all arguments being invoked (which could present a information leak). |
| `stdout` | Path to the file where stdout will be logged. If not specified, stdout will be written to babysitters' stdout. |
| `stderr` | Path to the file where stderr will be logged. If not specified, stderr will be written to babysitters' stderr. |
| `restart-delay-ms` | Number of milliseconds to delay between restarting the child daemon. Defaults to `2000`. |
| `webhook-script` | Path to shell script to be invoked when a web request is recieved at `/webhook/<webhook-token>`. |
| `webhook-token` | Secret component of the webhook path. |
| `dir` | Working directory of the daemon to be launched. Defaults to the current working directory. |

#### Example

```shell
./babysitter \
  --status-serv :7000 \
  --stdout /var/log/main_serv_out \
  --webhook-script /etc/onGithubPush.sh \
  --webhook-token superDuperSecretToken \
  /bin/mySecretServer --environment prod
```

In the above configuration, `babysitter` will invoke `/bin/mySecretServer --environment prod`, saving stdout to `/var/log/main_serv_out`. The webUI will be accessible on port 7000, and HTTP requests will `server:7000/webhook/superDuperSecretToken` invoke `/etc/onGithubPush.sh`.

As per defaults

1. The webUI will not show arguments to /bin/mySecretServer, the webhook script, or the token
2. The delay between invocations of `/bin/mySecretServer` (when it exits) is 2000ms.
3. The working directory of `/bin/mySecretServer` will default to the same as babysitter's.

## Webhook

The webhook is enabled when `webhook-token`, `webhook-script` and `status-serv` are specified.

Web requests to `/webhook/<webhook-token-value>` will result in bash invoking the script at `webhook-script`. The first parameter will be to a file which contains the HTTP body submitted with the request (empty file for any HTTP request whose method is not `POST`).

If the script exits with an exit code of 0, the daemon will be terminated and restarted.
