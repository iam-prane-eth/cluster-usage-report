package main

import (
	"os"
)

func PrepareDir(dir string) error {
	_, err := os.Stat(dir)
	if err != nil {
		// create dir if not exist
		if os.IsNotExist(err) {
			if err := os.Mkdir(dir, 0777); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
