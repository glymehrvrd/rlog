package rlog

import (
	"strings"
	"fmt"
	"path"
	"runtime"
)

func callerLine() string {
	var s string
	ptr := make([]uintptr, 5)
	n := runtime.Callers(2, ptr)
	if n >= 2 {
		f := runtime.FuncForPC(ptr[1])
		fileName, fileLine := f.FileLine(ptr[1])
		names := strings.Split(f.Name(), ".")
		s = fmt.Sprintf("%s:%d:%s", path.Base(fileName), fileLine, names[len(names)-1])
	}

	return s
}

func Debug(args ...interface{}) {
	if logLevel > kLogDebug {
		return
	}
	if logFilePath {
		grlog.Print("["+callerLine()+"]", "[DEBUG]", fmt.Sprintln(args...))
	} else {
		grlog.Print(args...)
	}
}

func Debugln(args ...interface{}) {
	if logLevel > kLogDebug {
		return
	}
	if logFilePath {
		grlog.Println("["+callerLine()+"]", "[DEBUG]", fmt.Sprintln(args...))
	} else {
		grlog.Println(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logLevel > kLogDebug {
		return
	}
	if logFilePath {
		grlog.Printf("[%s][DEBUG] %s", callerLine(), fmt.Sprintf(format, args...))
	} else {
		grlog.Printf(format, args...)
	}
}

func Error(args ...interface{}) {
	if logLevel > kLogError {
		return
	}
	if logFilePath {
		grlog.Print("["+callerLine()+"]", "[ERROR]", fmt.Sprintln(args...))
	} else {
		grlog.Print(args...)
	}
}

func Errorln(args ...interface{}) {
	if logLevel > kLogError {
		return
	}
	if logFilePath {
		grlog.Println("["+callerLine()+"]", "[ERROR]", fmt.Sprintln(args...))
	} else {
		grlog.Println(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if logLevel > kLogError {
		return
	}
	if logFilePath {
		grlog.Printf("[%s][ERROR] %s", callerLine(), fmt.Sprintf(format, args...))
	} else {
		grlog.Printf(format, args...)
	}
}
