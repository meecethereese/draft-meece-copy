package osutil

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/Azure/draft/pkg/config"
)

// Exists returns whether the given file or directory exists or not.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// SymlinkWithFallback attempts to symlink a file or directory, but falls back to a move operation
// in the event of the user not having the required privileges to create the symlink.
func SymlinkWithFallback(oldname, newname string) (err error) {
	err = os.Symlink(oldname, newname)
	if runtime.GOOS == "windows" {
		// If creating the symlink fails on Windows because the user
		// does not have the required privileges, ignore the error and
		// fall back to renaming the file.
		//
		// ERROR_PRIVILEGE_NOT_HELD is 0x522:
		// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681385(v=vs.85).aspx
		if lerr, ok := err.(*os.LinkError); ok && lerr.Err == syscall.Errno(0x522) {
			err = os.Rename(oldname, newname)
		}
	}
	return
}

// EnsureDirectory checks if a directory exists and creates it if it doesn't
func EnsureDirectory(dir string) error {
	if fi, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("could not create %s: %s", dir, err)
		}
	} else if !fi.IsDir() {
		return fmt.Errorf("%s must be a directory", dir)
	}

	return nil
}

// EnsureFile checks if a file exists and creates it if it doesn't
func EnsureFile(file string) error {
	fi, err := os.Stat(file)
	if err != nil {
		f, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("could not create %s: %s", file, err)
		}
		defer f.Close()
	} else if fi.IsDir() {
		return fmt.Errorf("%s must not be a directory", file)
	}

	return nil
}

type TemplateWriter interface {
	WriteFile(string, []byte, os.FileMode) error
	EnsureDirectory(string) error
}

type LocalFSWriter struct{}

func (w *LocalFSWriter) WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
func (w *LocalFSWriter) EnsureDirectory(path string) error {
	return EnsureDirectory(path)
}

func CopyDir(
	fileSys fs.FS,
	src, dest string,
	config *config.DraftConfig,
	customInputs map[string]string,
	templateWriter TemplateWriter) error {
	files, err := fs.ReadDir(fileSys, src)
	if err != nil {
		return err
	}

	for _, f := range files {

		if f.Name() == "draft.yaml" {
			continue
		}

		srcPath := src + "/" + f.Name()
		destPath := dest + "/" + f.Name()

		if f.IsDir() {
			if err = templateWriter.EnsureDirectory(destPath); err != nil {
				return err
			}
			if err = CopyDir(fileSys, srcPath, destPath, config, customInputs, templateWriter); err != nil {
				return err
			}
		} else {
			fileString, err := handleTemplateReplacement(fileSys, srcPath, customInputs)
			if err != nil {
				return err
			}

			fileName := checkNameOverrides(f.Name(), srcPath, destPath, config)
			if err = templateWriter.WriteFile(fmt.Sprintf("%s/%s", dest, fileName), []byte(fileString), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func CopyDirToFileMap(
	fileSys fs.FS,
	src, dest string,
	config *config.DraftConfig,
	customInputs map[string]string,
) (map[string][]byte, error) {

	fileMap := map[string][]byte{}
	files, err := fs.ReadDir(fileSys, src)
	if err != nil {
		return nil, err
	}

	for _, f := range files {

		if f.Name() == "draft.yaml" {
			continue
		}

		srcPath := src + "/" + f.Name()
		destPath := dest + "/" + f.Name()

		if f.IsDir() {
			fileMap, err = CopyDirToFileMap(fileSys, srcPath, destPath, config, customInputs)
			if err != nil {
				return nil, err
			}
		} else {
			fileString, err := handleTemplateReplacement(fileSys, srcPath, customInputs)
			if err != nil {
				return nil, err
			}

			fileName := checkNameOverrides(f.Name(), srcPath, destPath, config)
			fileMap[fmt.Sprintf("%s/%s", dest, fileName)] = fileString
		}
	}
	return fileMap, nil

}

func handleTemplateReplacement(fileSys fs.FS, srcPath string, customInputs map[string]string) ([]byte, error) {
	file, err := fs.ReadFile(fileSys, srcPath)
	if err != nil {
		return nil, err
	}

	fileString := string(file)

	for oldString, newString := range customInputs {
		log.Debugf("replacing %s with %s", oldString, newString)
		fileString = strings.ReplaceAll(fileString, "{{"+oldString+"}}", newString)
	}

	return []byte(fileString), nil
}

func checkNameOverrides(fileName, srcPath, destPath string, config *config.DraftConfig) string {
	if config != nil {
		log.Debugf("checking name override for srcPath: %s, destPath: %s", srcPath, destPath)
		if prefix := config.GetNameOverride(fileName); prefix != "" {
			log.Debugf("overriding file: %s with prefix: %s", destPath, prefix)
			fileName = fmt.Sprintf("%s%s", prefix, fileName)
		}
	}
	return fileName
}
