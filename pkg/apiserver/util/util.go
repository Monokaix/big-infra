package util

import "time"

func StrToTime(strTime string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", strTime, time.Local)
}