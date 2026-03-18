package fileops

import (
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Path    string
	Content string
	IsDir   bool
}

func Read(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Write(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func List(dir string, recursive bool) ([]FileInfo, error) {
	var files []FileInfo
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info := FileInfo{
			Path:  filepath.Join(dir, e.Name()),
			IsDir: e.IsDir(),
		}
		files = append(files, info)
		if recursive && e.IsDir() {
			sub, _ := List(info.Path, true)
			files = append(files, sub...)
		}
	}
	return files, nil
}

func Search(dir, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		if strings.Contains(strings.ToLower(filepath.Base(path)), strings.ToLower(pattern)) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func Grep(dir, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".exe" || ext == ".bin" || ext == ".png" || ext == ".jpg" {
			return nil
		}
		content, err := Read(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
				matches = append(matches, path+":"+string(rune('0'+i/10))+string(rune('0'+i%10))+" "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	return matches, err
}
