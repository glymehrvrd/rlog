package rlog

import (
	"fmt"
	"strconv"
	"strings"
	"path"
	"os"
	"log"
)

func max(nslice []int) int {
	maxn := 0
	for i := range nslice {
		if nslice[i] > maxn {
			maxn = nslice[i]
		}
	}
	return maxn
}

func contains(nslice []int, n int) bool {
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
			if !contains(numbers, kFileIndexNewStart+tmpTime) {
				return kFileIndexNewStart + tmpTime
			}
			tmpTime++
		}
	}
	return 1
}

type RollLog struct {
	LogDir      string
	LogPrefix   string
	currentSize int
	f           *os.File
}

func (r *RollLog) mkLogDir() {
	if r.LogDir == "" {
		r.LogDir = LogDir
	}
	if r.LogPrefix == "" {
		r.LogPrefix = kRlogFilePrefix
	}
	err := os.MkdirAll(r.LogDir, 0777)
	if err != nil {
		fmt.Printf("Rlog mkdir:%s, err:%s\n", LogDir, err.Error())
	}
}

func (r *RollLog) logFileName(n int) string {
	return r.LogDir + "." + strconv.Itoa(n)
}

func (r *RollLog) isRlogFileName(name string) (bool, int) {
	if !strings.HasPrefix(name, r.LogPrefix) {
		return false, 0
	}
	if len(name) == len(r.LogPrefix)+1 {
		return false, 0
	} else if len(name) > len(r.LogPrefix) {
		if string(name[len(r.LogPrefix):len(r.LogPrefix)+1]) != "." {
			return false, 0
		}
		i, err := strconv.Atoi(string(name[len(r.LogPrefix)+1:]))
		if err != nil || i <= 0 {
			return false, 0
		}
		return true, i
	} else {
		return true, 0
	}
}

func (r *RollLog) chooseFile() (string, error) {
	var oldestInfo os.FileInfo
	var newestInfo os.FileInfo
	var count int
	var numbers []int
	var err error
	var tmpDir *os.File

	tmpDir, err = os.Open(r.LogDir)
	if err != nil {
		fmt.Printf("chooseFile, open dir:%s, err:%s\n", r.LogDir, err.Error())
		return r.LogPrefix, err
	}
	defer tmpDir.Close()

	var subNames []string
	subNames, err = tmpDir.Readdirnames(-1)
	if err != nil {
		fmt.Printf("chooseFile, read dir:%s, err:%s\n", r.LogDir, err.Error())
		return r.LogPrefix, err
	}

	var fileName string
	for i := range subNames {
		var info os.FileInfo
		fileName = path.Join(r.LogDir, subNames[i])
		info, err = os.Stat(fileName)
		if err != nil {
			fmt.Printf("chooseFile, stat file:%s, err:%s\n", fileName, err.Error())
			continue
		}
		if info.IsDir() {
			continue
		}

		is, number := r.isRlogFileName(info.Name())
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
		return r.LogDir, nil
	}
	if count < logFileMaxNum {
		if newestInfo.Size() < int64(logFileMaxSize) {
			return newestInfo.Name(), nil
		} else {
			return r.logFileName(appropriateNextNumber(numbers)), nil
		}
	} else if count == logFileMaxNum {
		if newestInfo.Size() > int64(logFileMaxSize) {
			err = os.Remove(r.LogDir + "/" + oldestInfo.Name())
			if err != nil {
				fmt.Printf("rm file:%s, err:%s\n", oldestInfo.Name(), err.Error())
				return r.logFileName(appropriateNextNumber(numbers)), err
			}
		}
		return r.logFileName(appropriateNextNumber(numbers)), nil
	} else {
		// count > logFileMaxNum
		err = os.Remove(r.LogDir + kSep + oldestInfo.Name())
		if err != nil {
			fmt.Printf("rm file:%s, err:%s\n", oldestInfo.Name(), err.Error())
			return r.logFileName(appropriateNextNumber(numbers)), err
		}
		return r.logFileName(appropriateNextNumber(numbers)), nil
	}
}

func (r *RollLog) checkSizeAndCreateFile() error {
	var fileName string
	var info os.FileInfo
	var err error

	openLogFile := func() error {
		r.f, err = os.OpenFile(path.Join(r.LogDir, r.LogPrefix),
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

	rotateLog := func() {
		fileName, err = r.chooseFile()
		if err != nil {
			fmt.Printf("chooseFile failed, err[%s]", err.Error())
		}
		os.Rename(path.Join(r.LogDir, r.LogPrefix), path.Join(r.LogDir, fileName))
	}

	if r.f == nil {
		return openLogFile()
	}
	if r.currentSize > logFileMaxSize {
		r.f.Close()
		r.f = nil
		rotateLog()
		return openLogFile()

	}
	return nil
}

func (r *RollLog) Write(p []byte) (n int, err error) {
	onceMkLogDir.Do(r.mkLogDir)
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

func NewRollLog(dir, prefix string) *log.Logger {
	return log.New(&RollLog{
		LogDir:    dir,
		LogPrefix: prefix,
	}, "", log.LstdFlags)
}
