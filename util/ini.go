package util

import (
	"gopkg.in/ini.v1"
)

func Ini(ininame string) (map[string]string, error) {
	cfg, err := ini.Load("config/" + ininame)
	if err != nil {
		return nil, err
	}
	return cfg.Section("").KeysHash(), nil
}
