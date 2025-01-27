package tests

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	gno "github.com/gnolang/gno/pkgs/gnolang"
)

func TestFileStr(t *testing.T) {
	filePath := filepath.Join(".", "files", "str.gno")
	runFileTest(t, filePath, true)
}

// Bootstrapping test files from tests/files/*.gno,
// which primarily uses native stdlib shims.
func TestFiles1(t *testing.T) {
	syncWanted = false // Output assert mode
	baseDir := filepath.Join(".", "files")
	runFileTests(t, baseDir, true)
}

// Like TestFiles1(), but with more full-gno stdlib packages.
func TestFiles2(t *testing.T) {
	syncWanted = false // Output assert mode
	baseDir := filepath.Join(".", "files2")
	runFileTests(t, baseDir, false)
}

func TestChallenges(t *testing.T) {
	syncWanted = false // Output assert mode
	baseDir := filepath.Join(".", "challenges")
	runFileTests(t, baseDir, false)
}

func runFileTests(t *testing.T, baseDir string, nativeLibs bool) {
	t.Helper()

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".gno" {
			continue
		}
		if testing.Short() && strings.Contains(file.Name(), "_long") {
			t.Log(fmt.Sprintf("skipping test %s in short mode.", file.Name()))
			continue
		}
		file := file
		t.Run(file.Name(), func(t *testing.T) {
			runFileTest(t, filepath.Join(baseDir, file.Name()), nativeLibs)
		})
	}
}

func runFileTest(t *testing.T, path string, nativeLibs bool) {
	t.Helper()

	var logger loggerFunc
	if gno.IsDebug() && testing.Verbose() {
		logger = t.Log
	}
	err := RunFileTest("..", path, nativeLibs, logger)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}
