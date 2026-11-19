package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type projectMetadata struct {
	Root          string
	Name          string
	Hash          string
	RepositoryURL string
	Branch        string
}

// resolveProject discovers both ordinary .git directories and linked
// worktrees (.git files with a gitdir pointer) without executing git or adding
// a native dependency.
func resolveProject(inputPath string) (projectMetadata, error) {
	root := inputPath
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	worktreeRoot, gitDir, commonDir, err := discoverGit(root)
	if err != nil {
		return projectMetadata{}, err
	}
	if worktreeRoot != "" {
		root = worktreeRoot
	}
	identity := "path\x00" + root
	remote := ""
	branch := ""
	if gitDir != "" {
		identity = "git\x00" + commonDir
		remote = readRemoteOrigin(filepath.Join(commonDir, "config"))
		branch = readBranch(filepath.Join(gitDir, "HEAD"))
	}
	repositoryURL := sanitizeRepositoryURL(remote)
	name := filepath.Base(root)
	if remote != "" {
		identity = "repository\x00" + repositoryIdentity(remote)
		if remoteName := repositoryName(remote); remoteName != "" {
			name = remoteName
		}
	} else if commonDir != "" && filepath.Base(commonDir) == ".git" {
		name = filepath.Base(filepath.Dir(commonDir))
	}
	name = safeProjectName(name)
	digest := sha256.Sum256([]byte("gritloom-project-v1\x00" + identity))
	return projectMetadata{Root: root, Name: name, Hash: hex.EncodeToString(digest[:]), RepositoryURL: repositoryURL, Branch: branch}, nil
}

func discoverGit(root string) (worktreeRoot, gitDir, commonDir string, err error) {
	current := root
	for {
		marker := filepath.Join(current, ".git")
		if info, statErr := os.Stat(marker); statErr == nil {
			if info.IsDir() {
				gitDir = marker
			} else {
				contents, readErr := readSmallFile(marker, 4096)
				if readErr != nil {
					return "", "", "", fmt.Errorf("read worktree metadata: %w", readErr)
				}
				line := strings.TrimSpace(string(contents))
				if !strings.HasPrefix(line, "gitdir:") {
					return "", "", "", fmt.Errorf("invalid worktree metadata")
				}
				gitDir = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(current, gitDir)
				}
				gitDir = filepath.Clean(gitDir)
			}
			commonDir = gitDir
			if contents, readErr := readSmallFile(filepath.Join(gitDir, "commondir"), 4096); readErr == nil {
				commonDir = strings.TrimSpace(string(contents))
				if commonDir == "" {
					return "", "", "", fmt.Errorf("invalid git common directory")
				}
				if !filepath.IsAbs(commonDir) {
					commonDir = filepath.Join(gitDir, commonDir)
				}
				commonDir = filepath.Clean(commonDir)
			}
			return current, gitDir, commonDir, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", "", nil
		}
		current = parent
	}
}

func readBranch(headPath string) string {
	contents, err := readSmallFile(headPath, 4096)
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(contents))
	if !strings.HasPrefix(head, "ref: refs/heads/") {
		return ""
	}
	branch := strings.TrimPrefix(head, "ref: refs/heads/")
	if len(branch) > 200 || !utf8.ValidString(branch) || strings.IndexFunc(branch, unicode.IsControl) >= 0 {
		return ""
	}
	return branch
}

func readRemoteOrigin(configPath string) string {
	contents, err := readSmallFile(configPath, 1024*1024)
	if err != nil {
		return ""
	}
	inOrigin := false
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(line[1 : len(line)-1])
			inOrigin = len(section) >= len(`remote ""`) && strings.EqualFold(section[:len("remote")], "remote") && strings.Contains(section, `"origin"`)
			continue
		}
		if !inOrigin {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "url") {
			value = strings.TrimSpace(value)
			if utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0 {
				return value
			}
			return ""
		}
	}
	return ""
}

func sanitizeRepositoryURL(remote string) string {
	u, err := url.Parse(strings.TrimSpace(remote))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), ".git")
	result := u.String()
	if len(result) > 500 || strings.IndexFunc(result, unicode.IsControl) >= 0 {
		return ""
	}
	return result
}

func repositoryIdentity(remote string) string {
	remote = strings.TrimSpace(remote)
	if sanitized := sanitizeRepositoryURL(remote); sanitized != "" {
		return strings.ToLower(sanitized)
	}
	if u, err := url.Parse(remote); err == nil && u.Hostname() != "" && strings.EqualFold(u.Scheme, "ssh") {
		return strings.ToLower(u.Hostname()) + "/" + strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	}
	// SCP-style git remotes (git@host:owner/repository.git).
	if at := strings.LastIndex(remote, "@"); at >= 0 {
		remainder := remote[at+1:]
		if colon := strings.Index(remainder, ":"); colon > 0 {
			return strings.ToLower(remainder[:colon]) + "/" + strings.Trim(strings.TrimSuffix(remainder[colon+1:], ".git"), "/")
		}
	}
	return remote
}

func repositoryName(remote string) string {
	identity := repositoryIdentity(remote)
	if index := strings.IndexAny(identity, "?#"); index >= 0 {
		identity = identity[:index]
	}
	identity = strings.TrimRight(identity, "/")
	identity = strings.TrimSuffix(identity, ".git")
	if identity == "" {
		return ""
	}
	if slash := strings.LastIndexAny(identity, "/:"); slash >= 0 {
		identity = identity[slash+1:]
	}
	return identity
}

func safeProjectName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == string(filepath.Separator) || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		value = "project"
	}
	for len(value) > 80 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func readSmallFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > maximum {
		return nil, fmt.Errorf("metadata file is too large")
	}
	return contents, nil
}
