package lib

import "os"

func CreatePath(path string) (err error) {
	if !PathExists(path) {
		err = os.MkdirAll(path, os.ModePerm)
	}
	return
}

func PathExists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if os.IsNotExist(err) {
		return false
	}
	return true
}
