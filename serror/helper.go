package serror

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

const (
	appEnvKey = "APP_ENV"
	envLocal  = "local"
)

func isLocal() bool {
	v := os.Getenv(appEnvKey)
	if v == "" {
		v = envLocal
	}
	return strings.ToLower(v) == envLocal
}

func printErr(m string) {
	fmt.Fprintln(os.Stderr, m)
}

func exit() {
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)
}

func getPath(val string) string {
	for _, v := range rootPaths {
		if strings.HasPrefix(val, v) {
			n := len(v)
			if n >= len(val) {
				return val
			}
			return val[n:]
		}
	}
	return val
}
