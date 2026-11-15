package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Tokens never enter report JSON, Git repositories, or frontend bundles.
// Platform-specific implementations provide OS keychain or per-user storage.
type tokenStore interface {
	Load() (string, error)
	Save(string) error
	Delete() error
	Description() string
}

var lichessTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func validateLichessToken(token string) error {
	if len(token) > 2048 || !lichessTokenPattern.MatchString(token) {
		return errors.New("invalid Lichess token: expected only letters, digits or underscores")
	}
	return nil
}

func activeToken(s tokenStore) (token, source string, err error) {
	if env := strings.TrimSpace(os.Getenv("LICHESS_TOKEN")); env != "" {
		if err := validateLichessToken(env); err != nil {
			return "", "", errors.New("LICHESS_TOKEN environment variable is invalid")
		}
		return env, "environment", nil
	}
	token, err = s.Load()
	if err != nil {
		return "", "", err
	}
	if token != "" && validateLichessToken(token) != nil {
		return "", "", errors.New("saved Lichess credential is invalid")
	}
	if token != "" {
		return token, s.Description(), nil
	}
	return "", "", nil
}

type fileTokenStore struct{ path string }

func defaultTokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user configuration directory: %w", err)
	}
	return filepath.Join(dir, "ChessGrowth", "lichess-token"), nil
}
func (s fileTokenStore) Description() string { return "user settings file" }
func (s fileTokenStore) Load() (string, error) {
	info, err := os.Lstat(s.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("credential file is not a regular file")
	}
	if info.Size() > 4096 {
		return "", errors.New("credential file is unexpectedly large")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
func (s fileTokenStore) Save(token string) error {
	if err := validateLichessToken(token); err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Refuse to replace a symlink, and write atomically with owner-only permissions.
	if info, err := os.Lstat(s.path); err == nil && !info.Mode().IsRegular() {
		return errors.New("credential path is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(dir, ".lichess-token-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.WriteString(token + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	return nil
}
func (s fileTokenStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// errorTokenStore retains configuration errors instead of silently bypassing persistence.
type errorTokenStore struct{ err error }

func (s errorTokenStore) Load() (string, error) { return "", s.err }
func (s errorTokenStore) Save(string) error     { return s.err }
func (s errorTokenStore) Delete() error         { return s.err }
func (s errorTokenStore) Description() string   { return "unavailable" }
