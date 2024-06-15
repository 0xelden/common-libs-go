package logger

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/0xelden/common-libs-go/serror"
)

type LogInterceptor interface {
	Translate(lvl ErrorLevel, obj interface{}) string
	Process(lvl ErrorLevel, msg string)
}

type defaultInterceptorObj struct{}

func (defaultInterceptorObj) Translate(lvl ErrorLevel, obj interface{}) string {
	return DefaultTranslate(lvl, obj)
}

func (defaultInterceptorObj) Process(lvl ErrorLevel, msg string) {
	DefaultProcess(lvl, msg)
}

func DefaultInterceptor() LogInterceptor {
	return defaultInterceptorObj{}
}

func DefaultTranslate(lvl ErrorLevel, obj interface{}) string {
	m, m2 := DefaultTransform(lvl, obj)

	if !isLocal() {
		m2 = m
	}

	ts := time.Now().Format("2006-01-02 15:04:05")

	type lvll struct {
		Label string
		Color serror.Color
	}

	lbl := "?"
	ls := map[ErrorLevel]lvll{
		ErrorLevelInfo:     {"INFO", serror.LIGHT_BLUE},
		ErrorLevelLog:      {"LOG", serror.LIGHT_GRAY},
		ErrorLevelWarning:  {"WARN", serror.LIGHT_YELLOW},
		ErrorLevelCritical: {"ERR", serror.RED},
	}
	if cur, ok := ls[lvl]; ok {
		lbl = cur.Label
		if isLocal() {
			lbl = serror.ApplyForeColor(lbl, cur.Color)
		}
	}

	return fmt.Sprintf("[%s] %s: %s", ts, lbl, m2)
}

func DefaultTransform(lvl ErrorLevel, obj interface{}) (plainMsg string, colorMsg string) {
	plainMsg = fmt.Sprintf("%v", obj)
	colorMsg = plainMsg

	switch lvl {
	case ErrorLevelCritical, ErrorLevelWarning:
		switch vx := obj.(type) {
		case serror.SError:
			plainMsg = vx.String()
			colorMsg = vx.ColoredString()

		case error:
			pc, fn, line, _ := runtime.Caller(4)
			plainMsg = fmt.Sprintf(serror.StandardFormat(), runtime.FuncForPC(pc).Name(), fn, line, plainMsg)
			colorMsg = fmt.Sprintf(serror.StandardColorFormat(), runtime.FuncForPC(pc).Name(), fn, line, colorMsg)
		}
	}

	return plainMsg, colorMsg
}

func DefaultProcess(lvl ErrorLevel, msg string) {
	if msg == "" {
		return
	}
	switch lvl {
	case ErrorLevelCritical, ErrorLevelWarning:
		fmt.Fprintln(os.Stderr, msg)
	default:
		fmt.Fprintln(os.Stdout, msg)
	}
}
