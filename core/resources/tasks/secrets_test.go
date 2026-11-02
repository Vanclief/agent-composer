package tasks

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestListSecrets(t *testing.T) {
	env := newTestEnv(t)

	values := []string{"ghp_listvalue1", "sk-ant-listvalue2", "custom-listvalue3"}
	content := "GITHUB_TOKEN=" + values[0] + "\nANTHROPIC_API_KEY='" + values[1] + "'\nCUSTOM=" + values[2] + "\n"
	err := os.WriteFile(env.api.secretsPath, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	list, err := env.api.ListSecrets(context.Background(), nil, &ListSecretsRequest{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"secrets":[` +
		`{"name":"ANTHROPIC_API_KEY","hosts":["api.anthropic.com"]},` +
		`{"name":"CUSTOM","hosts":[]},` +
		`{"name":"GITHUB_TOKEN","hosts":["api.github.com","github.com"]}],` +
		`"harnesses":[{"id":"codex_cli","reads":["CODEX_API_KEY"]},{"id":"claude_code","reads":["CLAUDE_CODE_OAUTH_TOKEN","ANTHROPIC_API_KEY"]},{"id":"pi","reads":[]}]}`
	if string(data) != want {
		t.Fatalf("json = %s, want %s", data, want)
	}

	for _, value := range values {
		if strings.Contains(string(data), value) {
			t.Fatalf("json %s contains a secret value", data)
		}
	}
}

func TestListSecretsBadMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the mode check is skipped on windows")
	}

	env := newTestEnv(t)

	err := os.Chmod(env.api.secretsPath, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.ListSecrets(context.Background(), nil, &ListSecretsRequest{})
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED || !strings.Contains(ez.ErrorMessage(err), "chmod 600") {
		t.Fatalf("expected ENOTAUTHORIZED asking for chmod 600, got %v", err)
	}
	if strings.Contains(ez.ErrorMessage(err), "ghp_value") {
		t.Fatalf("error %v contains a secret value", err)
	}
}
