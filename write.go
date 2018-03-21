// write
package rlog

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"runtime"
	"path"
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

var logFilePath bool = true
var logLevel int = kLogDebug
var LogDir string = "."
var logFileMaxSize int = kDefaultMaxFileSize
var logFileMaxNum int = kDefaultMaxFileNum
var onceMkLogDir sync.Once
var grlog *log.Logger
var gStatLog *StatLog

var statTemplateTitle string = `--------------Stat Info-------------------
Cmd	Total	MaxTime	AverageTime	[10)	[10,50)	[50,100)	[100,200)	[200,500)	[500)`
var statTemplateCmd string = `%v	%d	%d	%d	%d	%d	%d	%d	%d	%d`

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
	grlog = log.New(&rollLog{}, "", log.LstdFlags)
	gStatLog = NewStatLog(grlog)
}

func mkLogDir() {
	err := os.MkdirAll(LogDir, 0777)
	if err != nil {
		fmt.Printf("Rlog mkdir:%s, err:%s\n", LogDir, err.Error())
	}
}

func isRlogFileName(name string) (bool, int) {
	if !strings.HasPrefix(name, kRlogFilePrefix) {
		return false, 0
	}
	if len(name) == len(kRlogFilePrefix)+1 {
		return false, 0
	} else if len(name) > len(kRlogFilePrefix) {
		if string(name[len(kRlogFilePrefix):len(kRlogFilePrefix)+1]) != "." {
			return false, 0
		}
		i, err := strconv.Atoi(string(name[len(kRlogFilePrefix)+1:]))
		if err != nil || i <= 0 {
			return false, 0
		}
		return true, i
	} else {
		return true, 0
	}
}

func logFileName(n int) string {
	return kRlogFilePrefix + "." + strconv.Itoa(n)
}

func max(nslice []int) int {
	maxn := 0
	for i := range nslice {
		if nslice[i] > maxn {
			maxn = nslice[i]
		}
	}
	return maxn
}

func contain(nslice []int, n int) bool {
	for i := range nslice {
		if nslice[i] == n {
			return true
		}
	}
	return false
}

func appropriateNextNumber(numbers []int) int {
	if len(numbers) == 0 {
		return 0
	}
	nmax := max(numbers)
	if nmax < kMaxFileIndex {
		return nmax + 1
	} else {
		tmpTime := 0
		for tmpTime < (len(numbers) + 3) {
			if !contain(numbers, kFileIndexNewStart+tmpTime) {
				return kFileIndexNewStart + tmpTime
			}
			tmpTime++
		}
	}
	return 1
}

func choseFile(dir string) (string, error) {
	var oldestInfo os.FileInfo
	var newestInfo os.FileInfo
	var count int
	var numbers []int
	var err error
	var tmpDir *os.File

	tmpDir, err = os.Open(dir)
	if err != nil {
		fmt.Printf("choseFile, open dir:%s, err:%s\n", dir, err.Error())
		return kRlogFilePrefix, err
	}
	defer tmpDir.Close()

	var subNames []string
	subNames, err = tmpDir.Readdirnames(-1)
	if err != nil {
		fmt.Printf("choseFile, read dir:%s, err:%s\n", dir, err.Error())
		return kRlogFilePrefix, err
	}

	var fileName string
	for i := range subNames {
		var info os.FileInfo
		fileName = dir + kSep + subNames[i]
		info, err = os.Stat(fileName)
		if err != nil {
			fmt.Printf("choseFile, Stat file:%s, err:%s\n", fileName, err.Error())
			continue
		}
		if info.IsDir() {
			continue
		}

		is, number := isRlogFileName(info.Name())
		if !is {
			continue
		}

		numbers = append(numbers, number)
		count++
		if oldestInfo == nil || info.ModTime().Before(oldestInfo.ModTime()) {
			oldestInfo = info
		}
		if newestInfo == nil || info.ModTime().After(newestInfo.ModTime()) {
			newestInfo = info
		}
	}

	if oldestInfo == nil {
		return kRlogFilePrefix, nil
	}
	if count < logFileMaxNum {
		if newestInfo.Size() < int64(logFileMaxSize) {
			return newestInfo.Name(), nil
		} else {
			return logFileName(appropriateNextNumber(numbers)), nil
		}
	} else if count == logFileMaxNum {
		if newestInfo.Size() > int64(logFileMaxSize) {
			err = os.Remove(dir + "/" + oldestInfo.Name())
			if err != nil {
				fmt.Printf("rm file:%s, err:%s\n", oldestInfo.Name(), err.Error())
				return logFileName(appropriateNextNumber(numbers)), err
			}
		}
		return logFileName(appropriateNextNumber(numbers)), nil
	} else { // count > logFileMaxNum
		err = os.Remove(dir + kSep + oldestInfo.Name())
		if err != nil {
			fmt.Printf("rm file:%s, err:%s\n", oldestInfo.Name(), err.Error())
			return logFileName(appropriateNextNumber(numbers)), err
		}
		return logFileName(appropriateNextNumber(numbers)), nil
	}
}

type rollLog struct {
	currentSize int
	f           *os.File
}

func (r *rollLog) checkSizeAndCreateFile() error {
	var fileName string
	var info os.FileInfo
	var err error

	openLogFile := func() error {
		r.f, err = os.OpenFile(LogDir+kSep+kRlogFilePrefix,
			os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		info, err = r.f.Stat()
		if err != nil {
			r.f.Close()
			r.f = nil
			return err
		}
		r.currentSize = int(info.Size())
		return nil
	}

	changLogFile := func() {
		fileName, err = choseFile(LogDir)
		if err == nil {
			os.Rename(LogDir+kSep+kRlogFilePrefix, LogDir+kSep+fileName)
		} else {
			fmt.Printf("changLogFile err:%s\n", err.Error())
		}
	}

	if r.f == nil {
		return openLogFile()
	}
	if r.currentSize > logFileMaxSize {
		r.f.Close()
		r.f = nil
		changLogFile()
		return openLogFile()

	}
	return nil
}

func SetLogLevel(level int) {
	logLevel = level
}

func (r *rollLog) Write(p []byte) (n int, err error) {
	onceMkLogDir.Do(mkLogDir)
	err = r.checkSizeAndCreateFile()
	if err != nil {
		fmt.Printf("Rlog write checkFile err:%s\n", err.Error())
		return len(p), nil
	}
	n, err = r.f.Write(p)
	if err == nil {
		r.currentSize += n
	} else {
		r.f.Close()
		r.f = nil
	}
	return
}

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

type statInfo struct {
	lessThenTen            int
	ten2Fifty              int
	fifty2OneHundred       int
	oneHundred2TwoHundred  int
	twoHundred2FiveHundred int
	moreThanFiveHundred    int
	totalCount             int
	maxTime                int
	averageTime            float64
}

type statRecord struct {
	cmd  string
	cost int
}

type StatLog struct {
	// stat log
	statMap  map[string]*statInfo
	dataChan chan statRecord
	loger    *log.Logger
	closed   bool
	buf      bytes.Buffer
}

func (s *StatLog) Close() {
	if s.closed {
		return
	}
	close(s.dataChan)
	s.closed = true
}

func (s *StatLog) startStat() {
	var tmpInfo statRecord
	timer := time.NewTimer(time.Minute)
	for !s.closed {
		select {
		case tmpInfo = <-s.dataChan:
			{
				s.statAdd(tmpInfo.cmd, tmpInfo.cost)
			}
		case <-timer.C:
			{
				s.printStat()
				timer.Reset(time.Minute)
			}
		}
	}
}

func (s *StatLog) printStat() {
	if len(s.statMap) == 0 {
		return
	}
	s.buf.Reset()
	s.buf.WriteString(statTemplateTitle)
	for k, v := range s.statMap {
		s.buf.WriteString("\n")
		s.buf.WriteString(fmt.Sprintf(
			statTemplateCmd, k, v.totalCount, v.maxTime, int(v.averageTime),
			v.lessThenTen, v.ten2Fifty, v.fifty2OneHundred,
			v.oneHundred2TwoHundred, v.twoHundred2FiveHundred,
			v.moreThanFiveHundred))
	}
	s.loger.Println(s.buf.String())
	s.statMap = make(map[string]*statInfo)
}

func (s *StatLog) statAdd(cmd string, cost int) {
	var i *statInfo
	var ok bool
	i, ok = s.statMap[cmd]
	if !ok {
		i = &statInfo{}
		s.statMap[cmd] = i
		i.maxTime = cost
		i.averageTime = float64(cost)
	} else {
		if cost > i.maxTime {
			i.maxTime = cost
		}
		i.averageTime = (i.averageTime + float64(cost)) / 2
	}
	i.totalCount++
	switch {
	case cost < 10:
		{
			i.lessThenTen++
		}
	case cost >= 10 && cost < 50:
		{
			i.ten2Fifty++
		}
	case cost >= 50 && cost < 100:
		{
			i.fifty2OneHundred++
		}
	case cost >= 100 && cost < 200:
		{
			i.oneHundred2TwoHundred++
		}
	case cost >= 200 && cost < 500:
		{
			i.twoHundred2FiveHundred++
		}
	case cost > 500:
		{
			i.moreThanFiveHundred++
		}
	}
}

func (s *StatLog) SetLogger(loger *log.Logger) {
	s.loger = loger
}

func NewStatLog(loger *log.Logger) *StatLog {
	l := &StatLog{
		statMap:  make(map[string]*statInfo),
		closed:   false,
		dataChan: make(chan statRecord, 1024),
		loger:    loger,
	}
	go l.startStat()
	return l
}

func (s *StatLog) StatAdd(cmd string, cost int) {
	s.dataChan <- statRecord{
		cmd:  cmd,
		cost: cost,
	}
}

func StatAdd(cmd string, cost int) {
	gStatLog.StatAdd(cmd, cost)
}

func DefaultStatSetLogger(loger *log.Logger) {
	gStatLog.SetLogger(loger)
}
