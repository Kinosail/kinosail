package documentdb

import (
	"database/sql"
	"io/fs"
	"os"
)

type dependencies struct {
	mkdirAll func(string, fs.FileMode) error
	lstat    func(string) (fs.FileInfo, error)
	openDB   func(string, string) (*sql.DB, error)
	chmod    func(string, fs.FileMode) error
	remove   func(string) error
}

func systemDependencies() dependencies {
	return dependencies{
		mkdirAll: os.MkdirAll,
		lstat:    os.Lstat,
		openDB:   sql.Open,
		chmod:    os.Chmod,
		remove:   os.Remove,
	}
}
