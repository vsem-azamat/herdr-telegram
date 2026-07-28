// Command validate-docs checks the design-only repository baseline.
package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	markdownLink = regexp.MustCompile(`!?\[[^]]+\]\(([^)]*)\)`)
	botToken     = regexp.MustCompile(`[0-9]{8,10}:[A-Za-z0-9_-]{30,}(?:$|[^A-Za-z0-9_-])`)
	tokenAssign  = regexp.MustCompile(`(?i)\b(bot[_-]token|telegram[_-]token)\b\s*=\s*['"]?([^\s'"]{8,})`)
)

var required = []string{
	"README.md",
	"SECURITY.md",
	"AGENTS.md",
	"CONTRIBUTING.md",
	"LICENSE",
	"docs/architecture.md",
	"docs/decisions.md",
	"docs/technology.md",
	"docs/threat-model.md",
	"docs/operations.md",
	"docs/implementation-plan.md",
	"docs/references.md",
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fail(err)
	}

	var problems []string
	problems = append(problems, validateRequired(root)...)
	problems = append(problems, validateLinks(root)...)
	problems = append(problems, validateSecrets(root)...)
	problems = append(problems, validateDesignOnly(root)...)
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "documentation validation failed:")
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "- %s\n", problem)
		}
		os.Exit(1)
	}
	fmt.Println("documentation validation passed")
}

func repositoryRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("locate repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func candidateFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list repository files: %w", err)
	}
	parts := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			files = append(files, string(part))
		}
	}
	return files, nil
}

func validateRequired(root string) []string {
	var problems []string
	for _, name := range required {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || !info.Mode().IsRegular() {
			problems = append(problems, "missing required file: "+name)
		}
	}
	return problems
}

func validateLinks(root string) []string {
	var problems []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".hermes" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, match := range markdownLink.FindAllSubmatch(data, -1) {
			rawTarget := strings.TrimSpace(string(match[1]))
			if rawTarget == "" {
				problems = append(problems, fmt.Sprintf("%s: empty link target", relative(root, path)))
				continue
			}
			target := strings.Trim(strings.Fields(rawTarget)[0], "<>")
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			pathPart, unescapeErr := url.PathUnescape(strings.SplitN(target, "#", 2)[0])
			if unescapeErr != nil {
				problems = append(problems, fmt.Sprintf("%s: invalid encoded link target: %s", relative(root, path), target))
				continue
			}
			if pathPart == "" {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(pathPart)))
			relRoot, relErr := filepath.Rel(root, resolved)
			if relErr != nil || relRoot == ".." || strings.HasPrefix(relRoot, ".."+string(filepath.Separator)) {
				problems = append(problems, fmt.Sprintf("%s: link escapes repository: %s", relative(root, path), target))
				continue
			}
			if _, statErr := os.Stat(resolved); statErr != nil {
				problems = append(problems, fmt.Sprintf("%s: missing link target: %s", relative(root, path), target))
			}
		}
		return nil
	})
	if err != nil {
		problems = append(problems, "scan Markdown links: "+err.Error())
	}
	return problems
}

func validateSecrets(root string) []string {
	files, err := candidateFiles(root)
	if err != nil {
		return []string{err.Error()}
	}
	return validateSecretFiles(root, files)
}

func validateSecretFiles(root string, files []string) []string {
	var problems []string
	for _, name := range files {
		data, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			problems = append(problems, fmt.Sprintf("%s: cannot scan for secrets: %v", name, readErr))
			continue
		}
		if !utf8.Valid(data) {
			continue
		}
		problems = append(problems, secretProblems(name, data)...)
	}
	return problems
}

func secretProblems(name string, data []byte) []string {
	var problems []string
	for index, line := range bytes.Split(data, []byte{'\n'}) {
		text := string(line)
		if botToken.MatchString(text) || unsafeTokenAssignment(text) {
			problems = append(problems, fmt.Sprintf("%s:%d: possible Telegram token", name, index+1))
		}
	}
	return problems
}

func unsafeTokenAssignment(line string) bool {
	match := tokenAssign.FindStringSubmatch(line)
	if len(match) == 0 {
		return false
	}
	value := strings.ToUpper(strings.Trim(match[2], "<>[]{}"))
	return value != "REDACTED" && value != "EXAMPLE" && value != "PLACEHOLDER"
}

func validateDesignOnly(root string) []string {
	productRoots := []string{"cmd/herdr-telegram", "internal"}
	for _, name := range productRoots {
		path := filepath.Join(root, filepath.FromSlash(name))
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return []string{"design-only baseline unexpectedly contains product code: " + name}
		}
	}
	return nil
}

func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
