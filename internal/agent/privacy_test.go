package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

func TestResolverHashesProjectAndSanitizesMetadata(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src", "feature")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/private-branch\n")
	writeTestFile(t, filepath.Join(root, ".git", "config"), "[remote \"origin\"]\n\turl = https://user:secret@example.com/owner/repo.git?token=secret\n")
	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	raw := tracking.Event{
		ID: "0123456789abcdef", ProjectPath: nested, ProjectName: "untrusted",
		Source: tracking.SourceVSCode, Kind: tracking.KindSave,
		File: filepath.Join(root, "src", "feature", "main.go"), OccurredAt: now,
	}
	event, err := resolver.ResolveEvent(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(event.ProjectPath) {
		t.Fatalf("project_path is not exactly lowercase SHA-256: %q", event.ProjectPath)
	}
	if event.ProjectName != "repo" || event.Branch != "private-branch" || event.File != "src/feature/main.go" {
		t.Fatalf("unexpected inferred metadata: %#v", event)
	}
	if event.RepositoryURL != "https://example.com/owner/repo" {
		t.Fatalf("repository URL was not sanitized: %q", event.RepositoryURL)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{root, "user:secret", "token=secret", "untrusted"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("uploaded event contains %q: %s", forbidden, encoded)
		}
	}
}

func TestResolverDetectsLinkedGitWorktreesAsOneProject(t *testing.T) {
	parent := t.TempDir()
	mainRoot := filepath.Join(parent, "main")
	worktreeRoot := filepath.Join(parent, "feature-checkout")
	commonGit := filepath.Join(mainRoot, ".git")
	worktreeGit := filepath.Join(commonGit, "worktrees", "feature")
	for _, directory := range []string{worktreeRoot, worktreeGit, filepath.Join(mainRoot, "nested")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(commonGit, "config"), "[remote \"origin\"]\n url = git@example.com:owner/shared.git\n")
	writeTestFile(t, filepath.Join(commonGit, "HEAD"), "ref: refs/heads/main\n")
	writeTestFile(t, filepath.Join(worktreeRoot, ".git"), "gitdir: "+worktreeGit+"\n")
	writeTestFile(t, filepath.Join(worktreeGit, "commondir"), "../..\n")
	writeTestFile(t, filepath.Join(worktreeGit, "HEAD"), "ref: refs/heads/feature\n")

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	mainEvent, err := resolver.ResolveEvent(tracking.Event{ProjectPath: filepath.Join(mainRoot, "nested"), Source: "generic", Kind: "focus"}, now)
	if err != nil {
		t.Fatal(err)
	}
	featureEvent, err := resolver.ResolveEvent(tracking.Event{ProjectPath: worktreeRoot, Source: "generic", Kind: "focus"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if mainEvent.ProjectPath != featureEvent.ProjectPath || mainEvent.ProjectName != featureEvent.ProjectName {
		t.Fatalf("worktrees did not resolve to one stable project: %#v %#v", mainEvent, featureEvent)
	}
	if featureEvent.Branch != "feature" || featureEvent.RepositoryURL != "" {
		t.Fatalf("unexpected worktree metadata: %#v", featureEvent)
	}
}

func TestExcludedProjectPathIsDroppedBeforeQueueing(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveEvent(tracking.Event{ProjectPath: child, Source: "generic", Kind: "focus"}, time.Now())
	if err != ErrExcluded {
		t.Fatalf("error = %v, want ErrExcluded", err)
	}
}

func TestResolverRejectsUnsafeFilesAndBadTimestamps(t *testing.T) {
	root := t.TempDir()
	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, file := range []string{"../secret", "/etc/passwd", "src/../secret", `C:\private\secret`} {
		_, err := resolver.ResolveEvent(tracking.Event{ProjectPath: root, Source: "generic", Kind: "save", File: file}, now)
		if err == nil {
			t.Errorf("unsafe file %q was accepted", file)
		}
	}
	_, err = resolver.ResolveEvent(tracking.Event{ProjectPath: root, Source: "generic", Kind: "focus", OccurredAt: now.Add(11 * time.Minute)}, now)
	if err == nil {
		t.Fatal("timestamp beyond ten-minute future bound was accepted")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
