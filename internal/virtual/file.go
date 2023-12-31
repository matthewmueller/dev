package virtual

import (
	"io"
	"io/fs"
	"path"
	"time"
)

type File struct {
	Path    string
	Data    []byte
	Mode    fs.FileMode
	ModTime time.Time
	Entries []fs.DirEntry
}

var _ fs.DirEntry = (*File)(nil)

// Name of the entry. Implements the fs.DirEntry interface.
func (f *File) Name() string {
	return path.Base(f.Path)
}

// Returns true if entry is a directory. Implements the fs.DirEntry interface.
func (f *File) IsDir() bool {
	return f.Mode.IsDir()
}

// Returns the type of entry. Implements the fs.DirEntry interface.
func (f *File) Type() fs.FileMode {
	return f.Mode.Type()
}

// Returns the file info. Implements the fs.DirEntry interface.
func (f *File) Info() (fs.FileInfo, error) {
	return &fileInfo{
		path:    f.Path,
		mode:    f.Mode,
		modTime: f.ModTime,
		size:    int64(len(f.Data)),
	}, nil
}

// Open the file
func (f *File) Open() fs.File {
	return &openFile{f, 0}
}

type openFile struct {
	*File
	offset int64
}

var _ fs.File = (*openFile)(nil)
var _ io.ReadSeeker = (*openFile)(nil)
var _ fs.DirEntry = (*openFile)(nil)

func (f *openFile) Close() error {
	return nil
}

func (f *openFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.Data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.Path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openFile) Stat() (fs.FileInfo, error) {
	return f.Info()
}

func (f *openFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.Data))
	}
	if offset < 0 || offset > int64(len(f.Data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.Path, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}
