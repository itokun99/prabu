package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const installManifestName = "install.json"

type installManifest struct {
	Version           int    `json:"version"`
	ZeroClawPath      string `json:"zeroclaw_path"`
	ZeroClawInstalled bool   `json:"zeroclaw_installed"`
	OMOPath           string `json:"omo_path"`
	OMOInstalled      bool   `json:"omo_installed"`
	ServiceManaged    bool   `json:"service_managed"`
}

func installManifestPath(home string) string {
	return filepath.Join(home, ".config", "prabu", installManifestName)
}

func readInstallManifest(home string) (installManifest, error) {
	path := installManifestPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return installManifest{}, fmt.Errorf("read Prabu install manifest %s: %w", path, err)
	}
	var manifest installManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return installManifest{}, fmt.Errorf("decode Prabu install manifest %s: %w", path, err)
	}
	if manifest.Version != 1 {
		return installManifest{}, fmt.Errorf("unsupported Prabu install manifest version %d", manifest.Version)
	}
	return manifest, nil
}

func readInstallManifestIfPresent(home string) (installManifest, error) {
	manifest, err := readInstallManifest(home)
	if errors.Is(err, os.ErrNotExist) {
		return installManifest{}, nil
	}
	return manifest, err
}

func writeInstallManifest(home string, manifest installManifest) error {
	path := installManifestPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Prabu state directory: %w", err)
	}
	manifest.Version = 1
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Prabu install manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write Prabu install manifest: %w", err)
	}
	return nil
}
