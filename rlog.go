package rlog

import (
	"flag"
	"log"
	"os"
	"sync"
)

const (
	kMinlogLevel = 1
	kLogDebug    = 2
	kLogError    = 3
	kMaxLogLevel = 3

	kDefaultMaxFileSize = 104857600 // 100M
	kDefaultMaxFileNum  = 10
	kRlogFilePrefix     = "rlog.log"
	kMaxFileIndex       = int(2147483640)
	kFileIndexNewStart  = 10240
	kSep                = string(os.PathSeparator)
)

var logFilePath = true
var logLevel = kLogDebug
var LogDir = "."
var logFileMaxSize = kDefaultMaxFileSize
var logFileMaxNum = kDefaultMaxFileNum
var onceMkLogDir sync.Once
var grlog *log.Logger

func init() {
	flag.IntVar(&logLevel, "rlog_level", kLogError,
		"rlog level, from 1-3, debug=2, error=3 if level < current_level, the log will not dump2disk")
	flag.StringVar(&LogDir, "rlog_dir", ".", "dir to put log files")
	flag.IntVar(&logFileMaxSize, "rlog_size", kDefaultMaxFileSize, "max size of per log file")
	flag.IntVar(&logFileMaxNum, "rlog_num", kDefaultMaxFileNum, "max num of log file")
	flag.BoolVar(&logFilePath, "rlog_filepath", true, "support filepath and function name")
	if logFileMaxNum < 2 {
		logFileMaxNum = kDefaultMaxFileNum
	}
	if logFileMaxSize < 4096 {
		logFileMaxSize = kDefaultMaxFileSize
	}
	grlog = NewRollLog(LogDir, kRlogFilePrefix)
}

func SetLogLevel(level int) {
	logLevel = level
}
