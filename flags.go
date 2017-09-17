package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

var parsedArgs map[string]flag
var extraArgs []string

type flag struct {
	rawValue string
}

func init() {
	parsedArgs = map[string]flag{}
	hasSeenAdditionalFlags := false
	for i := 1; i < len(os.Args); i++ {
		if (i+1) < len(os.Args) && strings.HasPrefix(os.Args[i], "--") && !hasSeenAdditionalFlags {
			parsedArgs[os.Args[i][2:]] = flag{
				rawValue: os.Args[i+1],
			}
			i++
		} else {
			hasSeenAdditionalFlags = true
			extraArgs = append(extraArgs, os.Args[i])
		}
	}
}

func boolFlag(name string, def bool) bool {
	if named, namedExists := parsedArgs[name]; namedExists {
		return strings.ToLower(named.rawValue) == "true" || named.rawValue == "1"
	}
	return def
}

func strFlag(name string, def string) string {
	if named, namedExists := parsedArgs[name]; namedExists {
		return named.rawValue
	}
	return def
}

func intFlag(name string, def int) int {
	if named, namedExists := parsedArgs[name]; namedExists {
		parsed, err := strconv.Atoi(named.rawValue)
		if err != nil {
			return parsed
		}
	}
	return def
}

func flagExists(name string) bool {
	_, exists := parsedArgs[name]
	return exists
}

func verifyFlags() error {
	if len(extraArgs) == 0 {
		return errors.New("no command specified")
	}
	return nil
}
