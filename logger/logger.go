package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ErrorLevel string

type Mode int

const (
	ModeDaily Mode = 1 + iota
	ModeMonthly
	ModeYearly
	ModePermanent
)

const (
	ErrorLevelDebug    ErrorLevel = "debug"
	ErrorLevelLog      ErrorLevel = "log"
	ErrorLevelInfo     ErrorLevel = "info"
	ErrorLevelCritical ErrorLevel = "critical"
	ErrorLevelWarning  ErrorLevel = "warn"
)

type Options struct {
	Mode        Mode
	Path        string
	Writing     bool
	FileFormat  string
	Interceptor LogInterceptor
}

type loggerObj struct {
	sync.Mutex
	Path       string
	Writing    bool
	Mode       Mode
	FileFormat string

	_ready       bool
	_file        string
	_name        string
	_queues      []string
	_interceptor LogInterceptor
	_stream      *os.File
}

type Logger interface {
	Startup() error
	Info(msg interface{})
	Infof(msg string, args ...interface{})
	Log(msg interface{})
	Logf(msg string, args ...interface{})
	Warn(msg interface{})
	Warnf(msg string, args ...interface{})
	Err(msg interface{})
	Errf(msg string, args ...interface{})
	Panic(msg interface{})
	IsWriting() bool
	StartWriting()
	StopWriting()
}

func chains(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func isExists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

func Construct(opt Options) Logger {
	return &loggerObj{
		Mode:         opt.Mode,
		Path:         opt.Path,
		Writing:      opt.Writing,
		FileFormat:   chains(opt.FileFormat, "log-%v.log"),
		_ready:       false,
		_interceptor: opt.Interceptor,
	}
}

func (ox *loggerObj) Startup() error {
	var err error
	if ox.Writing {
		cur := time.Now()
		fv := map[string]string{
			"d": cur.Format("02"),
			"m": cur.Format("01"),
			"y": cur.Format("2006"),
			"h": cur.Format("15"),
			"i": cur.Format("04"),
			"s": cur.Format("05"),
			"v": "",
		}

		formt := ox.FileFormat

		switch ox.Mode {
		case ModeDaily:
			fv["v"] = cur.Format("20060102")
		case ModeMonthly:
			fv["v"] = cur.Format("200601")
		case ModeYearly:
			fv["v"] = cur.Format("2006")
		case ModePermanent:
			fv["v"] = ""
		}

		for k, v := range fv {
			formt = strings.ReplaceAll(formt, "%"+k, v)
		}

		if ox._name != formt {
			ox._name = formt
			ox._file = filepath.Join(ox.Path, ox._name)

			if !isExists(ox.Path) {
				err = os.MkdirAll(ox.Path, os.ModePerm)
				if err != nil {
					return err
				}
			}

			err = ox.open()
			if err != nil {
				return err
			}
		}
	}

	if !ox._ready {
		go func() {
			for {
				time.Sleep(3 * time.Second)
				ox.flush()
			}
		}()
	}

	ox._ready = true
	return err
}

func (ox *loggerObj) Info(msg interface{}) {
	lvl := ErrorLevelInfo
	m := ox._interceptor.Translate(lvl, msg)
	ox._interceptor.Process(lvl, m)
	_ = ox.write(m)
}

func (ox *loggerObj) Infof(msg string, args ...interface{}) {
	ox.Info(fmt.Sprintf(msg, args...))
}

func (ox *loggerObj) Log(msg interface{}) {
	lvl := ErrorLevelLog
	m := ox._interceptor.Translate(lvl, msg)
	ox._interceptor.Process(lvl, m)
	_ = ox.write(m)
}

func (ox *loggerObj) Logf(msg string, args ...interface{}) {
	ox.Log(fmt.Sprintf(msg, args...))
}

func (ox *loggerObj) Warn(msg interface{}) {
	lvl := ErrorLevelWarning
	m := ox._interceptor.Translate(lvl, msg)
	ox._interceptor.Process(lvl, m)
	_ = ox.write(m)
}

func (ox *loggerObj) Warnf(msg string, args ...interface{}) {
	ox.Warn(fmt.Sprintf(msg, args...))
}

func (ox *loggerObj) Err(msg interface{}) {
	lvl := ErrorLevelCritical
	m := ox._interceptor.Translate(lvl, msg)
	ox._interceptor.Process(lvl, m)
	_ = ox.write(m)
}

func (ox *loggerObj) Errf(msg string, args ...interface{}) {
	ox.Err(fmt.Sprintf(msg, args...))
}

func (ox *loggerObj) Panic(msg interface{}) {
	ox.Err(msg)
	exit()
}

func (ox *loggerObj) IsWriting() bool {
	return ox.Writing
}

func (ox *loggerObj) StopWriting() {
	ox.Writing = false
}

func (ox *loggerObj) StartWriting() {
	ox.Writing = true
}

func (ox *loggerObj) open() error {
	if !ox.Writing {
		return nil
	}

	ox.Lock()
	defer ox.Unlock()

	if ox._stream != nil {
		_ = ox._stream.Close()
	}

	var err error
	ox._stream, err = os.OpenFile(ox._file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return err
}

func (ox *loggerObj) write(m string) error {
	if !ox.Writing {
		return nil
	}
	if !ox._ready {
		return errors.New("logger not yet ready")
	}
	if m == "" {
		return nil
	}
	ox.Lock()
	ox._queues = append(ox._queues, m)
	ox.Unlock()
	return nil
}

func (ox *loggerObj) flush() error {
	if !ox.Writing {
		return nil
	}
	if err := ox.Startup(); err != nil {
		return err
	}

	ox.Lock()
	lists := ox._queues
	ox._queues = []string{}
	ox.Unlock()

	var err error
	defer func() {
		if err != nil {
			ox.Lock()
			ox._queues = append(lists, ox._queues...)
			ox.Unlock()
		}
	}()

	for _, v := range lists {
		_, err = ox._stream.WriteString(fmt.Sprintf("%s\n", v))
		if err != nil {
			_ = ox.open()
			return err
		}
	}

	if len(lists) > 0 {
		err = ox._stream.Sync()
	}
	return err
}
