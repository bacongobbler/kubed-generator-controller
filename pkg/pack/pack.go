package pack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Pack defines a Draft Starter Pack.
type Pack struct {
	// Files are the files inside the Pack that will be installed.
	Files map[string]io.ReadCloser
}

// SaveDir saves a pack as files in a directory.
func (p *Pack) SaveDir(dest string) error {
	for relPath, f := range p.Files {
		path := filepath.Join(dest, relPath)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			baseDir := filepath.Dir(path)
			if os.MkdirAll(baseDir, 0755) != nil {
				return fmt.Errorf("Error creating directory %v: %v", baseDir, err)
			}
			newfile, err := os.Create(path)
			if err != nil {
				return err
			}
			defer newfile.Close()
			defer f.Close()
			io.Copy(newfile, f)
		} else if err != nil {
			return err
		}
	}

	return nil
}
