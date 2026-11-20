package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type lifecycleFixture struct {
	configPath   string
	soulPath     string
	zeroPath     string
	omoPath      string
	unitPath     string
	dropInPath   string
	manifestPath string
	secretPath   string
}

func TestDoctor_reportsReadyForInstalledPrabuStack(t *testing.T) {
	cli := newPrabuCLI(t)
	fixture := cli.installLifecycleState(t)

	output, err := cli.run(t, prabuInvocation{
		args: []string{"doctor", "--home", cli.home},
	})
	if err != nil {
		t.Fatalf("prabu doctor failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "READY") {
		t.Fatalf("doctor did not report the ready stack:\n%s", output)
	}
	if _, err := os.Stat(fixture.zeroPath); err != nil {
		t.Errorf("ZeroClaw executable missing: %v", err)
	}
	if _, err := os.Stat(fixture.omoPath); err != nil {
		t.Errorf("OMO executable missing: %v", err)
	}
}

func TestDoctor_rejectsUnversionedInstallManifest(t *testing.T) {
	cli := newPrabuCLI(t)
	fixture := cli.installLifecycleState(t)
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(readTestFile(t, fixture.manifestPath), &manifest); err != nil {
		t.Fatalf("decode lifecycle manifest fixture: %v", err)
	}
	delete(manifest, "version")
	unversioned, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("encode unversioned lifecycle manifest: %v", err)
	}
	writeTestFile(t, fixture.manifestPath, unversioned, 0o600)

	output, err := cli.run(t, prabuInvocation{
		args: []string{"doctor", "--home", cli.home},
	})
	if err == nil {
		t.Fatalf("doctor accepted an unversioned install manifest:\n%s", output)
	}
	if !strings.Contains(output, "unsupported Prabu install manifest version 0") {
		t.Fatalf("doctor did not explain the unsupported manifest version:\n%s", output)
	}
}

func TestUpdate_preservesConfigurationSecretsAndWorkspace(t *testing.T) {
	cli := newPrabuCLI(t)
	fixture := cli.installLifecycleState(t)
	configBefore := readTestFile(t, fixture.configPath)
	secretBefore := readTestFile(t, fixture.secretPath)
	soulBefore := readTestFile(t, fixture.soulPath)

	output, err := cli.run(t, prabuInvocation{
		args: []string{"update", "--home", cli.home, "--yes"},
	})
	if err != nil {
		t.Fatalf("prabu update failed: %v\n%s", err, output)
	}
	for _, path := range []string{
		filepath.Join(cli.home, ".prabu-zero-updated"),
		filepath.Join(cli.home, ".prabu-omo-updated"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("upstream update did not produce state %s: %v", path, err)
		}
	}
	if got := readTestFile(t, fixture.configPath); !bytes.Equal(got, configBefore) {
		t.Error("update changed ZeroClaw config")
	}
	if got := readTestFile(t, fixture.secretPath); !bytes.Equal(got, secretBefore) {
		t.Error("update changed stored secret state")
	}
	if got := readTestFile(t, fixture.soulPath); !bytes.Equal(got, soulBefore) {
		t.Error("update changed agent workspace instructions")
	}
}

func TestUninstall_removesManagedRuntimeButPreservesUserData(t *testing.T) {
	cli := newPrabuCLI(t)
	fixture := cli.installLifecycleState(t)
	configBefore := readTestFile(t, fixture.configPath)
	soulBefore := readTestFile(t, fixture.soulPath)
	secretBefore := readTestFile(t, fixture.secretPath)

	output, err := cli.run(t, prabuInvocation{
		args: []string{"uninstall", "--home", cli.home, "--yes"},
	})
	if err != nil {
		t.Fatalf("prabu uninstall failed: %v\n%s", err, output)
	}
	for _, path := range []string{fixture.zeroPath, fixture.omoPath, fixture.unitPath, fixture.dropInPath, fixture.manifestPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("managed runtime path remains after uninstall: %s (%v)", path, err)
		}
	}
	if got := readTestFile(t, fixture.configPath); !bytes.Equal(got, configBefore) {
		t.Error("uninstall removed or changed ZeroClaw user config")
	}
	if got := readTestFile(t, fixture.soulPath); !bytes.Equal(got, soulBefore) {
		t.Error("uninstall removed or changed user workspace")
	}
	if got := readTestFile(t, fixture.secretPath); !bytes.Equal(got, secretBefore) {
		t.Error("uninstall removed or changed encrypted secret state")
	}
}

func (cli prabuCLI) installLifecycleState(t *testing.T) lifecycleFixture {
	t.Helper()
	paths := lifecycleFixture{
		configPath:   filepath.Join(cli.home, ".zeroclaw", "config.toml"),
		soulPath:     filepath.Join(cli.home, ".zeroclaw", "agents", "prabu", "workspace", "SOUL.md"),
		zeroPath:     filepath.Join(cli.home, ".cargo", "bin", "zeroclaw"),
		omoPath:      filepath.Join(cli.home, ".local", "bin", "omo"),
		unitPath:     filepath.Join(cli.home, ".config", "systemd", "user", "zeroclaw.service"),
		dropInPath:   filepath.Join(cli.home, ".config", "systemd", "user", "zeroclaw.service.d", "10-prabu-omo.conf"),
		manifestPath: filepath.Join(cli.home, ".config", "prabu", "install.json"),
		secretPath:   filepath.Join(cli.home, ".zeroclaw", "secrets", "telegram.enc"),
	}
	for _, directory := range []string{
		filepath.Dir(paths.configPath),
		filepath.Dir(paths.soulPath),
		filepath.Dir(paths.zeroPath),
		filepath.Dir(paths.omoPath),
		filepath.Dir(paths.unitPath),
		filepath.Dir(paths.dropInPath),
		filepath.Dir(paths.manifestPath),
		filepath.Dir(paths.secretPath),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create lifecycle fixture directory: %v", err)
		}
	}
	for source, destination := range map[string]string{
		filepath.Join(cli.bin, "zeroclaw"): paths.zeroPath,
		filepath.Join(cli.bin, "omo"):      paths.omoPath,
	} {
		content := readTestFile(t, source)
		if err := os.WriteFile(destination, content, 0o700); err != nil {
			t.Fatalf("install lifecycle fixture executable: %v", err)
		}
	}
	writeTestFile(t, paths.configPath, []byte(`schema_version = 4

[providers.models.ollama.prabu]
model = "llama3.1"

[risk_profiles.prabu]
level = "supervised"
workspace_only = true
allowed_commands = ["omo"]
auto_approve = ["shell"]
always_ask = []

[agents.prabu]
model_provider = "ollama.prabu"
risk_profile = "prabu"
channels = ["telegram.prabu"]

[channels.telegram.prabu]
enabled = true
mention_only = true
per_user_session = true

[tools]
optional = []

[shell_tool]
timeout_secs = 1800
`), 0o600)
	writeTestFile(t, paths.soulPath, []byte("operator-authored workspace data\n"), 0o600)
	writeTestFile(t, paths.unitPath, []byte("[Service]\nExecStart=/managed/zeroclaw\n"), 0o600)
	writeTestFile(t, paths.dropInPath, []byte(managedMarker+"\n[Service]\nEnvironment=PATH=%h/.local/bin\n"), 0o600)
	writeTestFile(t, paths.secretPath, []byte("enc:fixture\n"), 0o600)
	manifest, err := json.Marshal(struct {
		Version           int    `json:"version"`
		ZeroClawPath      string `json:"zeroclaw_path"`
		ZeroClawInstalled bool   `json:"zeroclaw_installed"`
		OMOPath           string `json:"omo_path"`
		OMOInstalled      bool   `json:"omo_installed"`
		ServiceManaged    bool   `json:"service_managed"`
	}{
		Version:           1,
		ZeroClawPath:      paths.zeroPath,
		ZeroClawInstalled: true,
		OMOPath:           paths.omoPath,
		OMOInstalled:      true,
		ServiceManaged:    true,
	})
	if err != nil {
		t.Fatalf("encode lifecycle manifest: %v", err)
	}
	writeTestFile(t, paths.manifestPath, manifest, 0o600)
	return paths
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func writeTestFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
