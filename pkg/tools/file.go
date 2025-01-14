package tools

import (
	"io/ioutil"
	"path/filepath"
	"time"
)

func GetLastestUpdateFile(dir string) (string, error) {
	result := ""
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var latestTime time.Time
	for idx, file := range files {
		if idx == 0 {
			latestTime = file.ModTime()
			result = file.Name()
		} else {
			if file.ModTime().After(latestTime) {
				latestTime = file.ModTime()
				result = file.Name()
			}
		}

	}

	return filepath.Base(result), nil
}
