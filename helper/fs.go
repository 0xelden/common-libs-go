package helper

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/0xelden/common-libs-go/serror"
)

func MoveFile(srcPath, dstPath string) serror.SError {
	// Ensure the destination directory exists
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return serror.Newf("could not create destination directory: %v", err)
	}

	// Move the file
	if err := os.Rename(srcPath, dstPath); err != nil {
		return serror.Newf("could not move file: %v", err)
	}

	return nil
}

// IsCliExists checks if a CLI command is available in the system.
func IsCliExists(cli string) bool {
	_, err := exec.LookPath(cli)
	return err == nil
}
