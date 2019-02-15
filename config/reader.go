package config

import "io/ioutil"

type Reader interface {
	ReadFile(filename string) ([]byte, error)
}

type FileReader struct {
}

func NewFileReader() Reader {
	return &FileReader{}
}

func (f *FileReader) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}
