package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func WatchInfoToKeyDiffs(filesystemPath string, w client.WatchInfo) (client.KeyDiff, error) {
	diff := client.KeyDiff{
		Inserts: make(map[string]string),
		Updates: make(map[string]string),
		Deletions: w.Deletions,
	}

	for key, val := range w.Upserts {
		_, err := os.Stat(filepath.Join())
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return diff, err
			}

			diff.Inserts[key] = val.Value
			continue
		}

		diff.Updates[key] = val.Value
	}

	return diff, nil
}

func ConvertFileMode(mode string) os.FileMode {
	conv, _ := strconv.ParseInt(mode, 8, 32)
	return os.FileMode(conv)
}

func EnsureFilesystemDir(filesystemPath string, permissions os.FileMode) error {
	_, err := os.Stat(filesystemPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		mkErr := os.MkdirAll(filesystemPath, permissions)
		if mkErr != nil {
			return errors.New(fmt.Sprintf("Error creating filesystem directory: %s", mkErr.Error()))
		}
	}

	return nil
}

func GetDirectoryContent(path string) (client.KeyInfoMap, error) {
	keys := client.KeyInfoMap(map[string]client.KeyInfo{})

	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			keys[path] = client.KeyInfo{
				Key: path,
				Value: string(content),
				Version: 0,
				CreateRevision: 0,
				ModRevision: 0,
				Lease: 0,
			}
		}

		return nil
	})

	return keys, err
}

func ApplyDiffToDirectory(path string, diff client.KeyDiff, filesPermission os.FileMode, dirPermission os.FileMode) error {
	for _, file := range diff.Deletions {
		fPath := filepath.Join(path, file)
		err := os.Remove(fPath)
		if err != nil {
			return err
		}
	}

	upsertFile := func(file string, content string) error {
		fPath := filepath.Join(path, file)
		fdir := filepath.Dir(fPath)
		mkdirErr := os.MkdirAll(fdir, dirPermission)
		if mkdirErr != nil {
			return mkdirErr
		}

		f, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, filesPermission)
		if err != nil {
			return err
		}

		err = f.Truncate(0)
		if err != nil {
			f.Close()
			return err
		}

		_, err = f.Write([]byte(content))
		if err != nil {
			f.Close()
			return err
		}

		if err := f.Close(); err != nil {
			return err
		}

		return nil
	}

	for file, content := range diff.Inserts {
		err := upsertFile(file, content)
		if err != nil {
			return err
		}
	}

	for file, content := range diff.Updates {
		err := upsertFile(file, content)
		if err != nil {
			return err
		}
	}

	return nil
}