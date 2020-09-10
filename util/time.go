package util

import "time"

// ParseTime parses a string like "02.01.2006 15:04" to a unix timestamp.
func ParseTime(ts string) (int64, error) {
	t, err := time.ParseInLocation("02.01.2006 15:04", ts, time.Local)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}
