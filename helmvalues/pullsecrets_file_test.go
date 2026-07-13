package helmvalues

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParsePullSecretsFile tests the ParsePullSecretsFile function
func TestParsePullSecretsFile(t *testing.T) {
	t.Run("valid file with multiple entries", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "secrets.json")
		content := `{"pullSecrets": [{"registry": "docker.io", "secretName": "regcred"}, {"registry": "ghcr.io", "secretName": "ghcr-cred"}]}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := ParsePullSecretsFile(path)
		if err != nil {
			t.Fatalf("ParsePullSecretsFile() unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("ParsePullSecretsFile() returned nil")
		}
		if g, w := got.Get("docker.io"), "regcred"; g != w {
			t.Errorf("PullSecrets.Get(\"docker.io\") = %q, want %q", g, w)
		}
		if g, w := got.Get("ghcr.io"), "ghcr-cred"; g != w {
			t.Errorf("PullSecrets.Get(\"ghcr.io\") = %q, want %q", g, w)
		}
	})

	t.Run("valid file with empty pullSecrets list", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.json")
		content := `{"pullSecrets": []}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := ParsePullSecretsFile(path)
		if err != nil {
			t.Fatalf("ParsePullSecretsFile() unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("ParsePullSecretsFile() returned nil")
		}
		if g, w := len(got), 0; g != w {
			t.Errorf("PullSecrets length = %d, want %d", g, w)
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := ParsePullSecretsFile("/nonexistent/path/secrets.json")
		if err == nil {
			t.Fatal("ParsePullSecretsFile() expected error for nonexistent file, got nil")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "invalid.json")
		content := `{invalid json}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := ParsePullSecretsFile(path)
		if err == nil {
			t.Fatal("ParsePullSecretsFile() expected error for invalid JSON, got nil")
		}
	})

	t.Run("malformed structure returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "malformed.json")
		content := `{"pullSecrets": "not-an-array"}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := ParsePullSecretsFile(path)
		if err == nil {
			t.Fatal("ParsePullSecretsFile() expected error for wrong structure, got nil")
		}
	})

	t.Run("entry missing fields returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "incomplete_entry.json")
		content := `{"pullSecrets": [{"registry": "docker.io"}]}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := ParsePullSecretsFile(path)
		if err == nil {
			t.Fatal("ParsePullSecretsFile() expected error for incomplete entry, got nil")
		}
	})
}
