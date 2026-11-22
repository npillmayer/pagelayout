package template

import (
	"io/fs"
	"os"
)

// IO abstracts the host environment used to locate template collections.
// Directory paths use the host OS's syntax; names within the returned filesystem
// use io/fs syntax. A nil IO passed to LoadFile selects the OS implementation.
// Custom implementations must support concurrent calls if shared by callers.
type IO interface {
	UserConfigDir() (string, error)
	DirFS(string) fs.FS
}

type systemIO struct{}

func (systemIO) UserConfigDir() (string, error) { return os.UserConfigDir() }
func (systemIO) DirFS(dir string) fs.FS         { return os.DirFS(dir) }
