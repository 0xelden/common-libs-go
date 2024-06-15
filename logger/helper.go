package logger

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/0xelden/common-libs-go/serror"
)

var interceptor LogInterceptor = DefaultInterceptor()

func Info(msg interface{}) {
	lvl := ErrorLevelInfo
	m := interceptor.Translate(lvl, msg)
	interceptor.Process(lvl, m)
}

func Infof(msg string, args ...interface{}) {
	Info(fmt.Sprintf(msg, args...))
}

func Log(msg interface{}) {
	lvl := ErrorLevelLog
	m := interceptor.Translate(lvl, msg)
	interceptor.Process(lvl, m)
}

func Logf(msg string, args ...interface{}) {
	Log(fmt.Sprintf(msg, args...))
}

func Warn(msg interface{}) {
	lvl := ErrorLevelWarning
	m := interceptor.Translate(lvl, msg)
	interceptor.Process(lvl, m)
}

func Warnf(msg string, args ...interface{}) {
	Warn(fmt.Sprintf(msg, args...))
}

func Err(msg interface{}) {
	lvl := ErrorLevelCritical
	m := interceptor.Translate(lvl, msg)
	interceptor.Process(lvl, m)
}

func Errf(msg string, args ...interface{}) {
	Err(fmt.Sprintf(msg, args...))
}

func Panic(msg interface{}) {
	Err(castToSError(msg, 1))
	exit()
}

func isLocal() bool {
	v := os.Getenv("APP_ENV")
	if v == "" {
		v = "local"
	}
	return strings.ToLower(v) == "local"
}

func exit() {
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)
}

func castToSError(obj interface{}, skip int) serror.SError {
	if cur, ok := obj.(serror.SError); ok {
		return cur
	}
	if cur, ok := obj.(error); ok {
		return serror.NewFromErrors(skip+1, cur)
	}
	return serror.Newsf(skip+1, "%+v", obj)
}
