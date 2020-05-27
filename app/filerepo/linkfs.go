package filerepo

import (
	"os"

	"github.com/spf13/afero"
)

// LinkFs provides afero.Fs methods and extends it with Link.
type LinkFs struct {
	afero.OsFs
}

// Link creates newname as a hard link to the oldname file.
// If there is an error, it will be of type *LinkError.
func (lfs *LinkFs) Link(oldname, newname string) error {
	return os.Link(oldname, newname)
}
