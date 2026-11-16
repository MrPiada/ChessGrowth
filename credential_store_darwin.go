//go:build darwin

package main

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const keychainService = "com.chessgrowth.lichess-token"

type macKeychainStore struct{}

func newTokenStore() tokenStore              { return macKeychainStore{} }
func (macKeychainStore) Description() string { return "macOS Keychain" }
func (macKeychainStore) Load() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "security", "find-generic-password", "-a", "ChessGrowth", "-s", keychainService, "-w").Output()
	if ctx.Err() != nil {
		return "", errors.New("macOS Keychain did not respond within 10 seconds; unlock Keychain Access and retry")
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 44 {
			return "", nil
		}
		return "", errors.New("cannot read ChessGrowth token from macOS Keychain")
	}
	return strings.TrimSpace(string(data)), nil
}
func (macKeychainStore) Save(token string) error {
	if err := validateLichessToken(token); err != nil {
		return err
	}
	// security(1) uses argv for -w; the token is transiently visible to local process inspectors.
	// It is not written to disk by ChessGrowth, logged, or transmitted to the frontend.
	if err := exec.Command("security", "add-generic-password", "-a", "ChessGrowth", "-s", keychainService, "-w", token, "-U").Run(); err != nil {
		return errors.New("could not save token to macOS Keychain (check Keychain access permissions)")
	}
	return nil
}
func (macKeychainStore) Delete() error {
	cmd := exec.Command("security", "delete-generic-password", "-a", "ChessGrowth", "-s", keychainService)
	if err := cmd.Run(); err != nil {
		if got, loadErr := (macKeychainStore{}).Load(); loadErr == nil && got == "" {
			return nil
		}
		return errors.New("could not remove token from macOS Keychain")
	}
	return nil
}
