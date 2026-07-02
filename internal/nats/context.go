package nats

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

type contextFile struct {
	URL   string `json:"url"`
	Creds string `json:"creds"`
}

// CLIContext holds connection settings from a NATS CLI context file.
type CLIContext struct {
	Name  string
	URL   string
	Creds string
}

// LoadContext reads a named NATS CLI context from the default config directory.
func LoadContext(name string) (CLIContext, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CLIContext{}, fmt.Errorf("context name is required")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return CLIContext{}, fmt.Errorf("invalid context name %q", name)
	}

	parent, err := configParentDir()
	if err != nil {
		return CLIContext{}, err
	}

	path := filepath.Join(parent, "nats", "context", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CLIContext{}, fmt.Errorf("unknown context %q", name)
		}
		return CLIContext{}, fmt.Errorf("read context %q: %w", name, err)
	}

	var raw contextFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return CLIContext{}, fmt.Errorf("parse context %q: %w", name, err)
	}

	return CLIContext{
		Name:  name,
		URL:   strings.TrimSpace(raw.URL),
		Creds: expandHomedir(strings.TrimSpace(raw.Creds)),
	}, nil
}

func configParentDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".config"), nil
}

func expandHomedir(path string) string {
	path = os.ExpandEnv(path)
	if len(path) == 0 || path[0] != '~' {
		return path
	}

	usr, err := user.Current()
	if err != nil {
		return path
	}
	return strings.Replace(path, "~", usr.HomeDir, 1)
}
