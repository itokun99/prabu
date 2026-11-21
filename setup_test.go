package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestReadTerminalToken(t *testing.T) {
	token, err := readTerminalToken(strings.NewReader("  test-token  \n"), io.Discard)
	if err != nil {
		t.Fatalf("readTerminalToken: %v", err)
	}
	if token != "test-token" {
		t.Fatalf("token = %q, want %q", token, "test-token")
	}

	if _, err = readTerminalToken(strings.NewReader("\n"), io.Discard); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("empty token error = %v, want must-not-be-empty", err)
	}
}

func TestConfigPatchTokenRouting(t *testing.T) {
	patch, err := configPatch(setupConfig{provider: "ollama", model: "llama3.1", botToken: "test-token"})
	if err != nil {
		t.Fatalf("configPatch: %v", err)
	}
	var operations []configPatchOperation
	if err = json.Unmarshal(patch, &operations); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	foundToken, foundEnabled := false, false
	for _, op := range operations {
		if op.Path == "/channels/telegram/prabu/bot_token" {
			foundToken = true
			var value string
			if err = json.Unmarshal(op.Value, &value); err != nil || value != "test-token" {
				t.Fatalf("bot_token op = %s, want string test-token", op.Value)
			}
		}
		if op.Path == "/channels/telegram/prabu/enabled" {
			foundEnabled = true
		}
	}
	if !foundToken || !foundEnabled {
		t.Fatalf("token patch missing: token=%v enabled=%v", foundToken, foundEnabled)
	}

	patch, err = configPatch(setupConfig{provider: "ollama", model: "llama3.1"})
	if err != nil {
		t.Fatalf("configPatch without token: %v", err)
	}
	if strings.Contains(string(patch), "test-token") || strings.Contains(string(patch), "/channels/telegram/prabu/bot_token") {
		t.Fatal("patch without token must not carry token material")
	}
}
