package engine

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
)

// sys_io.h

type directory_entry_type int

const (
	et_file directory_entry_type = iota
	et_dir
)

type directory_entry struct {
	name  string
	size  uint64
	type_ directory_entry_type
}

func Sys_IO_Create_Directory(path string) {
	os.Mkdir(path, 0777)
}

// Returns 1 when the whole directory was read, 0 when the callback stopped
// it or the directory could not be read. On Windows a missing directory
// counts as read, as FindFirstFile reports it.
func Sys_IO_Read_Dir(basedir, subdir string, callback func(de *directory_entry) bool) int {
	dir := basedir
	if subdir != "" {
		dir += "/" + subdir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, fs.ErrNotExist) {
			return 1
		}
		return 0
	}
	for _, e := range entries {
		info, err := os.Stat(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		de := directory_entry{name: e.Name(), size: uint64(info.Size())}
		if subdir != "" {
			de.name = subdir + "/" + e.Name()
		}
		if info.IsDir() {
			de.type_ = et_dir
		}
		if !callback(&de) {
			return 0
		}
	}
	return 1
}

func Sys_IO_Path_Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// rename() of the C runtime: on Windows it fails when the target exists
func Sys_rename(oldpath, newpath string) error {
	if runtime.GOOS == "windows" {
		if _, err := os.Lstat(newpath); err == nil {
			return fs.ErrExist
		}
	}
	return os.Rename(oldpath, newpath)
}
