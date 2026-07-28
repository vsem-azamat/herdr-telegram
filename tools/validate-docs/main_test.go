package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsafeTokenAssignment(t *testing.T) {
	t.Parallel()

	secretAssignment := "BOT_TOKEN=" + "abcdefghijklmnop"
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "secret", line: secretAssignment, want: true},
		{name: "token file", line: "BOT_TOKEN_FILE=/run/secrets/telegram", want: false},
		{name: "redacted", line: "telegram_token=[REDACTED]", want: false},
		{name: "placeholder", line: "bot-token=<PLACEHOLDER>", want: false},
		{name: "embedded placeholder word", line: "bot_token=" + "realEXAMPLEsecret", want: true},
		{name: "regex declaration", line: `botToken = regexp.MustCompile("pattern")`, want: false},
		{name: "unrelated", line: "timeout=30s", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := unsafeTokenAssignment(test.line); got != test.want {
				t.Fatalf("unsafeTokenAssignment(%q) = %v, want %v", test.line, got, test.want)
			}
		})
	}
}

func TestSecretProblems(t *testing.T) {
	t.Parallel()

	directToken := "12345678:" + strings.Repeat("a", 30)
	longPrefix := strings.Repeat("x", 128<<10)
	assignment := "BOT_TOKEN=" + "abcdefghijklmnop"
	data := []byte(directToken + "\n" + longPrefix + "\n" + assignment + "\n")
	problems := secretProblems("fixture.txt", data)
	if len(problems) != 2 {
		t.Fatalf("secretProblems() = %v, want two findings", problems)
	}
	if problems[0] != "fixture.txt:1: possible Telegram token" {
		t.Fatalf("first finding = %q", problems[0])
	}
	if problems[1] != "fixture.txt:3: possible Telegram token" {
		t.Fatalf("second finding = %q", problems[1])
	}
}

func TestDirectTelegramTokenPattern(t *testing.T) {
	t.Parallel()

	base := "12345678:" + strings.Repeat("a", 29)
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "alphanumeric ending", text: base + "a", want: true},
		{name: "hyphen ending", text: base + "-", want: true},
		{name: "inside Bot API URL", text: "https://api.telegram.org/bot" + base + "-/getMe", want: true},
		{name: "too short", text: "12345678:" + strings.Repeat("a", 29), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := botToken.MatchString(test.text); got != test.want {
				t.Fatalf("botToken.MatchString(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestValidateSecretFilesReportsReadFailure(t *testing.T) {
	t.Parallel()

	problems := validateSecretFiles(t.TempDir(), []string{"missing.txt"})
	if len(problems) != 1 || !strings.Contains(problems[0], "cannot scan for secrets") {
		t.Fatalf("read-failure problems = %v, want one explicit failure", problems)
	}
}

func TestValidateRequired(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if problems := validateRequired(root); len(problems) != len(required) {
		t.Fatalf("empty required problems = %d, want %d", len(problems), len(required))
	}
	for _, name := range required {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if problems := validateRequired(root); len(problems) != 0 {
		t.Fatalf("complete required files reported problems: %v", problems)
	}
}

func TestValidateDesignOnly(t *testing.T) {
	t.Parallel()

	t.Run("empty baseline", func(t *testing.T) {
		root := t.TempDir()
		if problems := validateDesignOnly(root); len(problems) != 0 {
			t.Fatalf("empty baseline reported problems: %v", problems)
		}
	})

	for _, productRoot := range []string{"internal/domain", "cmd/herdr-telegram"} {
		productRoot := productRoot
		t.Run(productRoot, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(productRoot)), 0o755); err != nil {
				t.Fatal(err)
			}
			if problems := validateDesignOnly(root); len(problems) != 1 {
				t.Fatalf("product baseline problems = %v, want one", problems)
			}
		})
	}
}

func TestValidateLinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target file.md"), []byte("# Target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := "[encoded](target%20file.md)\n[fragment](target%20file.md#target)\n[external](https://example.com)\n"
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if problems := validateLinks(root); len(problems) != 0 {
		t.Fatalf("valid links reported problems: %v", problems)
	}

	tests := []struct {
		name string
		link string
	}{
		{name: "missing", link: "[missing](nope.md)\n"},
		{name: "empty", link: "[empty]()\n"},
		{name: "escape", link: "[escape](../outside.md)\n"},
		{name: "invalid encoding", link: "[invalid](bad%zz.md)\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(test.link), 0o600); err != nil {
				t.Fatal(err)
			}
			if problems := validateLinks(root); len(problems) != 1 {
				t.Fatalf("problems = %v, want one", problems)
			}
		})
	}
}
