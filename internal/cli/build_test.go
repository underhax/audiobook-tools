package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/underhax/audiobook-tools/internal/core"
)

func TestRunBuild(t *testing.T) {
	tempDir := t.TempDir()

	origCheckDeps := m4bCheckDependencies
	origExtractChapters := m4bExtractChaptersFromDir
	origBuild := m4bBuild
	origClean := m4bCleanIntermediateFiles
	defer func() {
		m4bCheckDependencies = origCheckDeps
		m4bExtractChaptersFromDir = origExtractChapters
		m4bBuild = origBuild
		m4bCleanIntermediateFiles = origClean
	}()

	m4bCheckDependencies = func() error { return nil }
	m4bExtractChaptersFromDir = func(_ string) ([]core.Chapter, error) { return nil, nil }
	m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, targetDir string, _ bool) (string, error) {
		return filepath.Join(targetDir, "out.m4b"), nil
	}
	m4bCleanIntermediateFiles = func(_ string) error { return nil }

	dirs := setupBuildTestDirs(t, tempDir)

	tests := []struct {
		name      string
		errStr    string
		dir       string
		setupMock func()
		extraArgs []string
		wantErr   bool
	}{
		{
			name:      "build help",
			extraArgs: []string{"-h"},
			wantErr:   false,
		},
		{
			name:      "build bad flag",
			extraArgs: []string{"-invalid-flag"},
			wantErr:   true,
			errStr:    "parse flags:",
		},
		{
			name:    "no dir flag",
			wantErr: true,
			errStr:  "-dir flag is required",
		},
		{
			name:      "success ID3 metadata",
			dir:       dirs.bookDir,
			extraArgs: []string{"-clean", "-debug"},
			wantErr:   false,
		},
		{
			name:    "success OPF metadata",
			dir:     dirs.opfDir,
			wantErr: false,
		},
		{
			name:    "ID3 empty tags",
			dir:     dirs.emptyMp3Dir,
			wantErr: false,
		},
		{
			name:    "ID3 only title",
			dir:     dirs.onlyTitleDir,
			wantErr: false,
		},
		{
			name:    "ID3 only author",
			dir:     dirs.onlyAuthorDir,
			wantErr: false,
		},
		{
			name:    "success Path metadata",
			dir:     dirs.emptyDir,
			wantErr: false,
		},
		{
			name:    "Path root metadata",
			dir:     "/",
			wantErr: false,
		},
		{
			name:    "dependency check error",
			dir:     dirs.emptyDir,
			wantErr: true,
			errStr:  "missing dependencies: mock check deps error",
			setupMock: func() {
				m4bCheckDependencies = func() error { return errors.New("mock check deps error") }
			},
		},
		{
			name:    "extract chapters error",
			dir:     dirs.emptyDir,
			wantErr: true,
			errStr:  "failed to extract chapters: mock extract error",
			setupMock: func() {
				m4bExtractChaptersFromDir = func(_ string) ([]core.Chapter, error) { return nil, errors.New("mock extract error") }
			},
		},
		{
			name:    "build error",
			dir:     dirs.emptyDir,
			wantErr: true,
			errStr:  "build m4b failed: mock build error",
			setupMock: func() {
				m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, _ string, _ bool) (string, error) {
					return "", errors.New("mock build error")
				}
			},
		},
		{
			name:      "clean error",
			dir:       dirs.emptyDir,
			extraArgs: []string{"-clean"},
			wantErr:   true,
			errStr:    "cleanup failed: mock clean error",
			setupMock: func() {
				m4bCleanIntermediateFiles = func(_ string) error { return errors.New("mock clean error") }
			},
		},
		{
			name:    "Path absolute error",
			dir:     dirs.emptyDir,
			wantErr: true,
			errStr:  "get absolute path: mock abs error",
			setupMock: func() {
				filepathAbs = func(_ string) (string, error) { return "", errors.New("mock abs error") }
			},
		},
		{
			name:    "unfinished downloads error",
			dir:     dirs.emptyDir,
			wantErr: false,
			errStr:  "",
			setupMock: func() {
				osReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{mockDirEntry{name: "part1.tmp"}}, nil
				}
			},
		},
		{
			name:    "unfinished downloads stderr write error",
			dir:     dirs.emptyDir,
			wantErr: true,
			errStr:  "write output: mock write error",
			setupMock: func() {
				osReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{mockDirEntry{name: "part2.tmp"}}, nil
				}
				stderrWriter = &buildBadWriter{failOnCount: 0}
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			origCheckDeps := m4bCheckDependencies
			origExtract := m4bExtractChaptersFromDir
			origBuild := m4bBuild
			origClean := m4bCleanIntermediateFiles
			origAbs := filepathAbs
			origReadDir := osReadDir
			origStderr := stderrWriter

			defer func() {
				m4bCheckDependencies = origCheckDeps
				m4bExtractChaptersFromDir = origExtract
				m4bBuild = origBuild
				m4bCleanIntermediateFiles = origClean
				filepathAbs = origAbs
				osReadDir = origReadDir
				stderrWriter = origStderr
			}()

			m4bCheckDependencies = func() error { return nil }
			m4bExtractChaptersFromDir = func(_ string) ([]core.Chapter, error) { return nil, nil }
			m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, targetDir string, _ bool) (string, error) {
				return filepath.Join(targetDir, "out.m4b"), nil
			}
			m4bCleanIntermediateFiles = func(_ string) error { return nil }
			filepathAbs = filepath.Abs

			if tt.setupMock != nil {
				tt.setupMock()
			}

			var args []string
			if tt.dir != "" {
				args = append(args, "-dir", tt.dir)
			}
			args = append(args, tt.extraArgs...)

			runBuildNormalTest(t, args, tt.wantErr, tt.errStr)
		})

		if !tt.wantErr && (len(tt.extraArgs) == 0 || tt.extraArgs[0] != "-h") {
			t.Run(tt.name+"_badwriter_0", func(t *testing.T) {
				var args []string
				if tt.dir != "" {
					args = append(args, "-dir", tt.dir)
				}
				args = append(args, tt.extraArgs...)
				runBuildBadWriterTest(t, args, 0)
			})
			t.Run(tt.name+"_badwriter_1", func(t *testing.T) {
				var args []string
				if tt.dir != "" {
					args = append(args, "-dir", tt.dir)
				}
				args = append(args, tt.extraArgs...)
				runBuildBadWriterTest(t, args, 1)
			})
			t.Run(tt.name+"_badwriter_2", func(t *testing.T) {
				var args []string
				if tt.dir != "" {
					args = append(args, "-dir", tt.dir)
				}
				args = append(args, tt.extraArgs...)
				runBuildBadWriterTest(t, args, 2)
			})
		}
	}
}

func runBuildNormalTest(t *testing.T, args []string, wantErr bool, errStr string) {
	var out bytes.Buffer
	err := RunBuild(args, &out)

	if (err != nil) != wantErr {
		t.Fatalf("RunBuild() error = %v, wantErr %v", err, wantErr)
	}
	if wantErr && !strings.Contains(err.Error(), errStr) {
		t.Errorf("RunBuild() error = %v, want err string %q", err, errStr)
	}
}

type buildBadWriter struct {
	failOnCount int
	count       int
}

func (w *buildBadWriter) Write(p []byte) (int, error) {
	if w.count >= w.failOnCount {
		return 0, errors.New("mock write error")
	}
	w.count++
	return len(p), nil
}

func runBuildBadWriterTest(t *testing.T, args []string, failOnCount int) {
	writer := &buildBadWriter{failOnCount: failOnCount}
	err := RunBuild(args, writer)
	if err == nil {
		if writer.count <= failOnCount {
			return
		}
		t.Errorf("expected write output error, got nil")
	} else if !strings.Contains(err.Error(), "write output") {
		t.Errorf("expected write output error, got %v", err)
	}
}

type buildTestDirs struct {
	bookDir       string
	emptyMp3Dir   string
	onlyTitleDir  string
	onlyAuthorDir string
	opfDir        string
	emptyDir      string
}

func setupBuildTestDirs(t *testing.T, tempDir string) buildTestDirs {
	t.Helper()

	var dirs buildTestDirs

	dirs.bookDir = filepath.Join(tempDir, "TestBookDir")
	if err := os.Mkdir(dirs.bookDir, 0o700); err != nil {
		t.Fatalf("failed to create book dir: %v", err)
	}

	mp3Path := filepath.Join(dirs.bookDir, "01-chapter.mp3")

	id3Header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 100}
	tit2Header := []byte{'T', 'I', 'T', '2', 0, 0, 0, 11, 0, 0}
	tit2Data := append([]byte{3}, []byte("Test Title")...)
	tpe1Header := []byte{'T', 'P', 'E', '1', 0, 0, 0, 12, 0, 0}
	tpe1Data := append([]byte{3}, []byte("Test Author")...)

	mp3Data := make([]byte, 0, len(id3Header)+len(tit2Header)+len(tit2Data)+len(tpe1Header)+len(tpe1Data))
	mp3Data = append(mp3Data, id3Header...)
	mp3Data = append(mp3Data, tit2Header...)
	mp3Data = append(mp3Data, tit2Data...)
	mp3Data = append(mp3Data, tpe1Header...)
	mp3Data = append(mp3Data, tpe1Data...)

	if err := os.WriteFile(mp3Path, mp3Data, 0o600); err != nil {
		t.Fatalf("failed to write mp3: %v", err)
	}

	dirs.emptyMp3Dir = filepath.Join(tempDir, "EmptyMp3Dir")
	if err := os.Mkdir(dirs.emptyMp3Dir, 0o700); err != nil {
		t.Fatalf("failed to create empty mp3 dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirs.emptyMp3Dir, "empty.mp3"), id3Header, 0o600); err != nil {
		t.Fatalf("failed to write empty mp3: %v", err)
	}

	dirs.onlyTitleDir = filepath.Join(tempDir, "OnlyTitleDir")
	if err := os.Mkdir(dirs.onlyTitleDir, 0o700); err != nil {
		t.Fatalf("failed to create only title dir: %v", err)
	}
	onlyTitleData := make([]byte, 0, len(id3Header)+len(tit2Header)+len(tit2Data))
	onlyTitleData = append(onlyTitleData, id3Header...)
	onlyTitleData = append(onlyTitleData, tit2Header...)
	onlyTitleData = append(onlyTitleData, tit2Data...)
	if err := os.WriteFile(filepath.Join(dirs.onlyTitleDir, "title.mp3"), onlyTitleData, 0o600); err != nil {
		t.Fatalf("failed to write only title mp3: %v", err)
	}

	dirs.onlyAuthorDir = filepath.Join(tempDir, "OnlyAuthorDir")
	if err := os.Mkdir(dirs.onlyAuthorDir, 0o700); err != nil {
		t.Fatalf("failed to create only author dir: %v", err)
	}
	onlyAuthorData := make([]byte, 0, len(id3Header)+len(tpe1Header)+len(tpe1Data))
	onlyAuthorData = append(onlyAuthorData, id3Header...)
	onlyAuthorData = append(onlyAuthorData, tpe1Header...)
	onlyAuthorData = append(onlyAuthorData, tpe1Data...)
	if err := os.WriteFile(filepath.Join(dirs.onlyAuthorDir, "author.mp3"), onlyAuthorData, 0o600); err != nil {
		t.Fatalf("failed to write only author mp3: %v", err)
	}

	dirs.opfDir = filepath.Join(tempDir, "OpfDir")
	if err := os.Mkdir(dirs.opfDir, 0o700); err != nil {
		t.Fatalf("failed to create opf dir: %v", err)
	}
	opfContent := `<?xml version="1.0" encoding="UTF-8"?><package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>OPF Title</dc:title></metadata></package>`
	if err := os.WriteFile(filepath.Join(dirs.opfDir, "metadata.opf"), []byte(opfContent), 0o600); err != nil {
		t.Fatalf("failed to write opf: %v", err)
	}

	dirs.emptyDir = filepath.Join(tempDir, "EmptyDir")
	if err := os.Mkdir(dirs.emptyDir, 0o700); err != nil {
		t.Fatalf("failed to create empty dir: %v", err)
	}

	return dirs
}
