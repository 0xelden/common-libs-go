package serror

import (
	"errors"
	"fmt"
	"runtime"

	gerr "github.com/go-errors/errors"
)

type SError interface {
	Error() string
	Cause() error
	Key() string
	Code() int
	Title() string
	Comments() string
	CommentStack() []string
	Callers() []uintptr
	StackFrames() []gerr.StackFrame
	Type() string
	File() string
	Line() int
	FN() string
	SetKey(key string)
	AddComments(msg ...string)
	AddCommentf(msg string, opts ...interface{})
	AddCommentsx(skip int, msg ...string)
	AddCommentfx(skip int, msg string, opts ...interface{})
	SetComments(note string)
	String() string
	SimpleString() string
	ColoredString() string
	Panic()
	Print()
	PrintWithColor()
	IsEqual(err error) bool
}

var rootPaths []string

func New(message string) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2, stack[:])
	return construct(stack[:length], 0, "-", errors.New(message), 0, "@")
}

func Newf(message string, args ...interface{}) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2, stack[:])
	return construct(stack[:length], 0, "-", fmt.Errorf(message, args...), 0, "@")
}

func Newc(message string, note string) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2, stack[:])
	return construct(stack[:length], 0, "-", errors.New(message), 0, note)
}

func Newsf(skip int, message string, args ...interface{}) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2+skip, stack[:])
	return construct(stack[:length], 0, "-", fmt.Errorf(message, args...), skip, "@")
}

func NewFromError(err error) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2, stack[:])
	return construct(stack[:length], 0, "-", err, 0, "@")
}

func NewFromErrorc(err error, note string) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2, stack[:])
	return construct(stack[:length], 0, "-", err, 0, note)
}

func NewFromErrors(skip int, err error) SError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(2+skip, stack[:])
	return construct(stack[:length], 0, "-", err, skip, "@")
}

func StandardFormat() string {
	return "In %s[%s:%d] %s%s"
}

func StandardColorFormat() string {
	frmt := ""
	frmt += ApplyForeColor("In", DARK_GRAY) + " "
	frmt += ApplyForeColor("%s", LIGHT_YELLOW)
	frmt += ApplyForeColor("[", DARK_GRAY)
	frmt += ApplyForeColor("%s:%d", MAGENTA)
	frmt += ApplyForeColor("]", DARK_GRAY)
	frmt += " %s%s"
	return frmt
}

func IsEqual(a error, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	if errx, ok := a.(SError); ok {
		a = errx.Cause()
	}
	if errx, ok := b.(SError); ok {
		b = errx.Cause()
	}
	return a.Error() == b.Error()
}
