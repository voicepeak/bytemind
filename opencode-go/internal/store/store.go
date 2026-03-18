package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

type Session struct {
	ID       string    `json:"id"`
	Dir      string    `json:"dir"`
	Messages []Message `json:"messages"`
	Created  time.Time `json:"created"`
}

type Store struct {
	path    string
	current *Session
}

func New() (*Store, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".opencode-go")
	os.MkdirAll(dir, 0755)
	s := &Store{path: dir}
	s.loadCurrent()
	return s, nil
}

func (s *Store) loadCurrent() {
	b, _ := os.ReadFile(filepath.Join(s.path, "current.json"))
	if len(b) > 0 {
		json.Unmarshal(b, &s.current)
	}
	if s.current == nil {
		s.current = &Session{
			ID:      "default",
			Dir:     ".",
			Created: time.Now(),
		}
	}
}

func (s *Store) Save() error {
	b, _ := json.MarshalIndent(s.current, "", "  ")
	return os.WriteFile(filepath.Join(s.path, "current.json"), b, 0644)
}

func (s *Store) SetDir(dir string) {
	s.current.Dir = dir
	s.Save()
}

func (s *Store) Dir() string {
	return s.current.Dir
}

func (s *Store) AddMessage(role, content string) {
	s.current.Messages = append(s.current.Messages, Message{
		Role:    role,
		Content: content,
		Time:    time.Now().Unix(),
	})
}

func (s *Store) GetHistory() []Message {
	return s.current.Messages
}

func (s *Store) ClearHistory() {
	s.current.Messages = nil
	s.Save()
}
