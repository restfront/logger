package logger

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fileRotator struct {
	path     string
	file     *os.File
	date     time.Time
	compress bool
	mu       sync.Mutex
}

var _ io.WriteCloser = (*fileRotator)(nil)

func (r *fileRotator) openNew(onDate time.Time) error {
	r.date = onDate

	if _, err := os.Stat(r.path); errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(r.path, 0777)
		if err != nil {
			return err
		}
	}

	filename := filepath.Join(r.path, r.date.Format("2006_01_02")+".log")

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	r.file = file

	return nil
}

func (r *fileRotator) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		if err := r.openNew(time.Now()); err != nil {
			return 0, err
		}
	}

	if r.needRotate() {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}

	return r.file.Write(p)
}

func (r *fileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return nil
	}

	if err := r.file.Sync(); err != nil {
		return err
	}

	if err := r.file.Close(); err != nil {
		return err
	}

	return nil
}

func (r *fileRotator) rotate() error {
	if err := r.file.Sync(); err != nil {
		return err
	}

	if err := r.file.Close(); err != nil {
		return err
	}

	if r.compress {
		go compressFile(r.file.Name())
	}

	if err := r.openNew(time.Now()); err != nil {
		return err
	}

	return nil
}

func (r *fileRotator) needRotate() bool {
	return r.date.Day() != time.Now().Day() || r.date.Month() != time.Now().Month() || r.date.Year() != time.Now().Year()
}

func compressFile(src string) {
	file, err := os.Open(src)
	if err != nil {
		return
	}
	defer file.Close()

	zipFile, err := os.Create(src + ".zip")
	if err != nil {
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	info, err := file.Stat()
	if err != nil {
		return
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return
	}

	_, err = io.Copy(writer, file)
	if err != nil {
		return
	}

	_ = file.Close()
	_ = os.Remove(src)
}
