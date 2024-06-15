package serror

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	gerr "github.com/go-errors/errors"
	"github.com/rivo/uniseg"
)

type serrorObj struct {
	err      error
	key      string
	comments []string
	code     int

	frames []gerr.StackFrame
	stack  []uintptr
}

func construct(stack []uintptr, code int, key string, err error, skip int, note string) SError {
	var (
		res    *serrorObj
		isSErr bool
	)

	if err != nil {
		if errx, ok := err.(SError); ok {
			isSErr = true
			res = &serrorObj{
				err:      errx.Cause(),
				key:      errx.Key(),
				code:     errx.Code(),
				comments: errx.CommentStack(),
			}
			for _, v := range errx.StackFrames() {
				res.stack = append(res.stack, v.ProgramCounter)
			}
			if key != "" && key != "-" {
				res.key = key
			}
			if code != 0 {
				res.code = code
			}
		}
	}

	if res == nil {
		res = &serrorObj{
			err:   err,
			key:   key,
			code:  code,
			stack: stack,
		}
	}

	if !isSErr && err != nil && note == "@" {
		note = err.Error()
	}

	if note != "" {
		res.AddCommentsx(skip+2, note)
	}
	return res
}

func (ox serrorObj) Error() string {
	return fmt.Sprintf("%v", ox.err)
}

func (ox serrorObj) Cause() error {
	return ox.err
}

func (ox serrorObj) Code() int {
	return ox.code
}

func (ox serrorObj) Key() string {
	return ox.key
}

func (ox serrorObj) Callers() []uintptr {
	return ox.stack
}

func (ox *serrorObj) StackFrames() []gerr.StackFrame {
	if ox.frames == nil {
		ox.frames = make([]gerr.StackFrame, len(ox.stack))
		for i, pc := range ox.stack {
			item := gerr.NewStackFrame(pc)
			item.File = getPath(item.File)
			ox.frames[i] = item
		}
	}
	return ox.frames
}

func (ox serrorObj) Type() string {
	return reflect.TypeOf(ox.err).String()
}

func (ox *serrorObj) File() string {
	frames := ox.StackFrames()
	if len(frames) > 0 {
		return frames[0].File
	}
	return ""
}

func (ox *serrorObj) Line() int {
	frames := ox.StackFrames()
	if len(frames) > 0 {
		return frames[0].LineNumber
	}
	return 0
}

func (ox *serrorObj) FN() string {
	frames := ox.StackFrames()
	if len(frames) > 0 {
		return frames[0].Name
	}
	return ""
}

func (ox serrorObj) Title() string {
	if len(ox.comments) > 0 {
		return ox.comments[0]
	}
	return ox.Error()
}

func (ox serrorObj) Comments() string {
	return strings.Join(ox.comments, ", ")
}

func (ox serrorObj) CommentStack() []string {
	return ox.comments
}

func (ox *serrorObj) SetKey(key string) {
	ox.key = key
}

func (ox *serrorObj) setRawComment(note string, skip int) {
	if ox.comments == nil {
		ox.comments = []string{}
	}

	if int64(uniseg.GraphemeClusterCount(note)) <= 0 {
		return
	}

	if len(ox.comments) <= 0 {
		ox.comments = append(ox.comments, strings.ToUpper(string(note[0]))+string(note[1:]))
		return
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	ox.comments = append(ox.comments, fmt.Sprintf("%s on [%s:%d]", note, getPath(file), line))
}

func (ox *serrorObj) SetComments(note string) {
	ox.setRawComment(note, 1)
}

func (ox *serrorObj) AddComments(msg ...string) {
	for _, v := range msg {
		ox.setRawComment(v, 1)
	}
}

func (ox *serrorObj) AddCommentf(msg string, opts ...interface{}) {
	ox.setRawComment(fmt.Sprintf(msg, opts...), 1)
}

func (ox *serrorObj) AddCommentsx(skip int, msg ...string) {
	for _, v := range msg {
		ox.setRawComment(v, 1+skip)
	}
}

func (ox *serrorObj) AddCommentfx(skip int, msg string, opts ...interface{}) {
	ox.setRawComment(fmt.Sprintf(msg, opts...), 1+skip)
}

func (ox serrorObj) String() string {
	return fmt.Sprintf(StandardFormat(), ox.fParams()...)
}

func (ox serrorObj) ColoredString() string {
	return fmt.Sprintf(StandardColorFormat(), ox.fParams()...)
}

func (ox serrorObj) SimpleString() string {
	msg := ox.Error()
	if len(ox.comments) > 0 {
		msg = fmt.Sprintf("%s, detail: %s", ox.Comments(), msg)
	}
	return msg
}

func (ox serrorObj) Panic() {
	defer exit()
	if isLocal() {
		ox.PrintWithColor()
		return
	}
	ox.Print()
}

func (ox serrorObj) Print() {
	printErr(ox.String())
}

func (ox serrorObj) PrintWithColor() {
	printErr(ox.ColoredString())
}

func (ox serrorObj) IsEqual(err error) bool {
	return IsEqual(ox.Cause(), err)
}

func (ox serrorObj) fParams() []interface{} {
	pars := []interface{}{
		ox.FN(),
		ox.File(),
		ox.Line(),
		"",
		ox.Error(),
	}
	if len(ox.comments) > 0 {
		pars[3] = fmt.Sprintf("%s, details: ", ox.Comments())
	}
	return pars
}
