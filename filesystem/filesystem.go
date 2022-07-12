package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"ferlab/configurations-auto-updater/model"
)

func ConvertFileMode(mode string) os.FileMode {
	conv, _ := strconv.ParseInt(mode, 8, 32)
	return os.FileMode(conv)
}

func EnsureFilesystemDir(filesystemPath string, permissions os.FileMode) error {
	_, err := os.Stat(filesystemPath)
	if err != nil &&  errors.Is(err, os.ErrNotExist) {
		mkErr := os.MkdirAll(filesystemPath, permissions)
		if mkErr != nil {
			return errors.New(fmt.Sprintf("Error creating filesystem directory: %s", mkErr.Error()))
		}
	}

	return nil
}

func GetDirectoryContent(path string) (map[string]model.KeyInfo, error) {
	keys := make(map[string]model.KeyInfo)

	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			keys[path] = model.KeyInfo{
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

func ApplyDiffToDirectory(path string, diff model.KeysDiff, filesPermission os.FileMode, dirPermission os.FileMode) error {
	for _, file := range diff.Deletions {
		fPath := filepath.Join(path, file)
		err := os.Remove(fPath)
		if err != nil {
			return err
		}
	}

	for file, content := range diff.Upserts {
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
	}

	return nil
}

/*func ListZonefiles(zonefilesPath string) ([]string, error) {
	zonefiles, err := ioutil.ReadDir(zonefilesPath)
	if err != nil {
		return []string{}, errors.New(fmt.Sprintf("Error listing zonefiles: %s", err.Error()))
	}

	result := make([]string, len(zonefiles))
	for idx, zonefile := range zonefiles {
		result[idx] = zonefile.Name()
	}
	return result, nil
}

func GetZonefileDeletions(newZonefiles map[string]string, PreExistingZonefiles []string) []string {
	deletions := []string{}
	for _, zonefile := range PreExistingZonefiles {
		if _, ok := newZonefiles[zonefile]; !ok {
			deletions = append(deletions, zonefile)
		}
	}

	return deletions
}

func DeleteZonefile(zonefilesPath string, zonefile string) error {
	err := os.Remove(path.Join(zonefilesPath, zonefile))
	if err != nil {
		return errors.New(fmt.Sprintf("Error deleting zonefile: %s", err.Error()))
	}

	return nil
}

func UpsertZonefile(zonefilesPath string, zonefile string, content string) error {
	err := ioutil.WriteFile(path.Join(zonefilesPath, zonefile), []byte(content), 0644)
	if err != nil {
		return errors.New(fmt.Sprintf("Error upserting zonefile: %s", err.Error()))
	}

	return nil
}

func ApplyZonefilesChanges(zonefilesPath string, upsertedZonefiles map[string]string, deletedZonefiles []string) error {
	for k, v := range upsertedZonefiles {
		err := UpsertZonefile(zonefilesPath, k, v)
		if err != nil {
			return err
		}
	}

	for _, v := range deletedZonefiles {
		err := DeleteZonefile(zonefilesPath, v)
		if err != nil {
			return err
		}
	}

	return nil
}*/