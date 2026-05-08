package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
	"github.com/shanmeiliu/rag-infra-go/internal/ingestion"
)

type IngestionService interface {
	Ingest(ctx context.Context, chunks []ingestion.InputChunk) error
}

type SourceCleaner interface {
	DeleteBySourceID(ctx context.Context, sourceID string) error
}

type Service struct {
	repo         *Repository
	ingestionSvc IngestionService
	cleaner      SourceCleaner
	uploadDir    string
}

type githubMarkdownFile struct {
	Path    string
	Content string
}

func NewService(repo *Repository, ingestionSvc IngestionService, cleaner SourceCleaner, uploadDir string) *Service {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &Service{
		repo:         repo,
		ingestionSvc: ingestionSvc,
		cleaner:      cleaner,
		uploadDir:    uploadDir,
	}
}

func (s *Service) EnsureUploadDir() error {
	return os.MkdirAll(s.uploadDir, 0o755)
}

func (s *Service) ListSources(ctx context.Context, limit int) ([]Source, error) {
	return s.repo.List(ctx, limit)
}

func (s *Service) HandleUploadedFile(
	ctx context.Context,
	user *auth.User,
	filename string,
	sourceType string,
	reader io.Reader,
) (*Source, error) {
	if err := s.EnsureUploadDir(); err != nil {
		return nil, err
	}

	sourceKey := uuid.NewString()
	safeName := sanitizeFilename(filename)
	if safeName == "" {
		safeName = "upload.txt"
	}

	targetPath := filepath.Join(s.uploadDir, sourceKey+"_"+safeName)
	out, err := os.Create(targetPath)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if _, err := out.Write(contentBytes); err != nil {
		return nil, err
	}

	extractedText, parserName, err := ExtractText(filename, contentBytes)
	if err != nil {
		return nil, err
	}

	sourceGroup := sourceGroupForType(sourceType)
	originValue := "upload"

	var createdBy *int64
	if user != nil {
		createdBy = &user.ID
	}

	src := &Source{
		SourceKey:  sourceKey,
		Name:       filename,
		SourceType: sourceType,
		Status:     "ready",
		Origin:     &originValue,
		FilePath:   stringPtr(targetPath),
		Metadata: map[string]any{
			"filename":             filename,
			"size":                 len(contentBytes),
			"kind":                 "upload",
			"parser":               parserName,
			"source_group":         sourceGroup,
			"extracted_char_count": len(extractedText),
		},
		CreatedByUserID: createdBy,
	}

	id, err := s.repo.Create(ctx, src)
	if err != nil {
		return nil, err
	}
	src.ID = id

	if strings.TrimSpace(extractedText) != "" {
		if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, sourceGroup, filename, filename, extractedText, map[string]any{
			"filename":             filename,
			"kind":                 "upload",
			"parser":               parserName,
			"extracted_char_count": len(extractedText),
		}); err != nil {
			return nil, err
		}
	}

	return src, nil
}

func (s *Service) HandleGithubRepo(
	ctx context.Context,
	user *auth.User,
	repoURL string,
	branch string,
	sourceType string,
	includePatterns []string,
) (*Source, error) {
	if strings.TrimSpace(repoURL) == "" {
		return nil, fmt.Errorf("repo_url is required")
	}

	mdFiles, normalizedRepo, resolvedBranch, err := fetchGithubMarkdownFiles(ctx, repoURL, branch, includePatterns)
	if err != nil {
		return nil, err
	}

	sourceKey := uuid.NewString()
	sourceGroup := sourceGroupForType(sourceType)

	var createdBy *int64
	if user != nil {
		createdBy = &user.ID
	}

	src := &Source{
		SourceKey:  sourceKey,
		Name:       normalizedRepo,
		SourceType: sourceType,
		Status:     "ready",
		Origin:     &repoURL,
		Metadata: map[string]any{
			"repo_url":         repoURL,
			"normalized_repo":  normalizedRepo,
			"branch":           resolvedBranch,
			"include_patterns": includePatterns,
			"ingested_content": "markdown_files",
			"markdown_count":   len(mdFiles),
			"kind":             "github",
			"source_group":     sourceGroup,
		},
		CreatedByUserID: createdBy,
	}

	id, err := s.repo.Create(ctx, src)
	if err != nil {
		return nil, err
	}
	src.ID = id

	if err := s.ingestGithubMarkdownFiles(ctx, src.SourceKey, src.SourceType, sourceGroup, repoURL, normalizedRepo, resolvedBranch, mdFiles); err != nil {
		return nil, err
	}

	return src, nil
}

func (s *Service) SyncSource(ctx context.Context, id int64) (*Source, error) {
	src, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.cleaner.DeleteBySourceID(ctx, src.SourceKey); err != nil {
		return nil, err
	}

	kind, _ := src.Metadata["kind"].(string)
	sourceGroup := sourceGroupForType(src.SourceType)

	switch kind {
	case "github":
		repoURL, _ := src.Metadata["repo_url"].(string)
		branch, _ := src.Metadata["branch"].(string)
		includePatterns := normalizeIncludePatterns(src.Metadata["include_patterns"])

		mdFiles, normalizedRepo, resolvedBranch, err := fetchGithubMarkdownFiles(ctx, repoURL, branch, includePatterns)
		if err != nil {
			return nil, err
		}

		if err := s.ingestGithubMarkdownFiles(ctx, src.SourceKey, src.SourceType, sourceGroup, repoURL, normalizedRepo, resolvedBranch, mdFiles); err != nil {
			return nil, err
		}

	case "upload":
		if src.FilePath == nil || strings.TrimSpace(*src.FilePath) == "" {
			return nil, fmt.Errorf("uploaded source has no file path")
		}

		contentBytes, err := os.ReadFile(*src.FilePath)
		if err != nil {
			return nil, err
		}

		extractedText, parserName, err := ExtractText(src.Name, contentBytes)
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(extractedText) != "" {
			if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, sourceGroup, src.Name, src.Name, extractedText, map[string]any{
				"filename":             src.Name,
				"kind":                 "upload",
				"parser":               parserName,
				"extracted_char_count": len(extractedText),
			}); err != nil {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("source kind does not support sync")
	}

	if err := s.repo.TouchUpdatedAt(ctx, id); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) DeleteSource(ctx context.Context, id int64) error {
	src, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.cleaner.DeleteBySourceID(ctx, src.SourceKey); err != nil {
		return err
	}

	if src.FilePath != nil && strings.TrimSpace(*src.FilePath) != "" {
		_ = os.Remove(*src.FilePath)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) ingestGithubMarkdownFiles(
	ctx context.Context,
	sourceID string,
	sourceType string,
	sourceGroup string,
	repoURL string,
	repoName string,
	branch string,
	files []githubMarkdownFile,
) error {
	for _, file := range files {
		if strings.TrimSpace(file.Content) == "" {
			continue
		}

		if err := s.ingestChunkedContent(ctx, sourceID, sourceType, sourceGroup, file.Path, file.Path, file.Content, map[string]any{
			"repo_url":  repoURL,
			"repo_name": repoName,
			"branch":    branch,
			"filepath":  file.Path,
			"kind":      "github",
			"parser":    "github_markdown",
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ingestChunkedContent(
	ctx context.Context,
	sourceID string,
	sourceType string,
	sourceGroup string,
	docName string,
	chunkIDPrefix string,
	content string,
	extra map[string]any,
) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	chunks := ChunkText(content, 1800)
	if len(chunks) == 0 {
		return nil
	}

	docID := "source-" + sourceID + "-" + sanitizeChunkIDPart(chunkIDPrefix)
	inputs := make([]ingestion.InputChunk, 0, len(chunks))

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("chunk-%s-%s-%d", sourceID, sanitizeChunkIDPart(chunkIDPrefix), i+1)

		metadata := map[string]any{
			"source_id":    sourceID,
			"source_group": sourceGroup,
			"source_type":  sourceType,
			"doc_name":     docName,
			"chunk_index":  i + 1,
			"chunk_count":  len(chunks),
		}

		for k, v := range extra {
			metadata[k] = v
		}

		inputs = append(inputs, ingestion.InputChunk{
			ChunkID:  chunkID,
			DocID:    docID,
			Content:  chunk,
			Metadata: metadata,
		})
	}

	return s.ingestionSvc.Ingest(ctx, inputs)
}

func sourceGroupForType(sourceType string) string {
	switch strings.TrimSpace(sourceType) {
	case "resume":
		return "resume"
	case "github_repo", "portfolio_project", "technical_project":
		return "repos"
	case "job_description", "job-desc":
		return "job-desc"
	case "notes", "document":
		return "notes"
	default:
		return "notes"
	}
}

func fetchGithubMarkdownFiles(ctx context.Context, repoURL, branch string, includePatterns []string) ([]githubMarkdownFile, string, string, error) {
	owner, repo, err := parseGithubRepo(repoURL)
	if err != nil {
		return nil, "", "", err
	}

	resolvedBranch := strings.TrimSpace(branch)
	if resolvedBranch == "" {
		resolvedBranch, err = fetchGithubDefaultBranch(ctx, owner, repo)
		if err != nil {
			return nil, "", "", err
		}
	}

	tree, err := fetchGithubTree(ctx, owner, repo, resolvedBranch)
	if err != nil {
		return nil, "", "", err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	files := make([]githubMarkdownFile, 0)

	for _, item := range tree.Tree {
		if item.Type != "blob" {
			continue
		}

		if !isMarkdownPath(item.Path) {
			continue
		}

		if len(includePatterns) > 0 && !matchesIncludePatterns(item.Path, includePatterns) {
			continue
		}

		content, err := fetchGithubFileContent(ctx, client, owner, repo, resolvedBranch, item.Path)
		if err != nil {
			return nil, "", "", err
		}

		if strings.TrimSpace(content) == "" {
			continue
		}

		files = append(files, githubMarkdownFile{
			Path:    item.Path,
			Content: content,
		})
	}

	if len(files) == 0 {
		return nil, "", "", fmt.Errorf("no markdown files found in GitHub repo")
	}

	return files, owner + "/" + repo, resolvedBranch, nil
}

type githubRepoResponse struct {
	DefaultBranch string `json:"default_branch"`
}

func fetchGithubDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rag-infra-go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch GitHub repo metadata: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload githubRepoResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	if strings.TrimSpace(payload.DefaultBranch) == "" {
		return "", fmt.Errorf("GitHub repo default branch is empty")
	}

	return payload.DefaultBranch, nil
}

type githubTreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
	Truncated bool `json:"truncated"`
}

func fetchGithubTree(ctx context.Context, owner, repo, branch string) (*githubTreeResponse, error) {
	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		owner,
		repo,
		url.PathEscape(branch),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rag-infra-go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch GitHub tree: status=%d body=%s", resp.StatusCode, string(body))
	}

	var tree githubTreeResponse
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, err
	}

	return &tree, nil
}

func fetchGithubFileContent(ctx context.Context, client *http.Client, owner, repo, branch, path string) (string, error) {
	escapedPath := escapeGithubContentPath(path)

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		owner,
		repo,
		escapedPath,
		url.QueryEscape(branch),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rag-infra-go")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch GitHub file %s: status=%d body=%s", path, resp.StatusCode, string(body))
	}

	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	if payload.Encoding != "base64" {
		return "", fmt.Errorf("unsupported GitHub file encoding for %s: %s", path, payload.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func escapeGithubContentPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func matchesIncludePatterns(path string, patterns []string) bool {
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	if path == "" {
		return false
	}

	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(strings.TrimSpace(pattern), "/")
		if pattern == "" {
			continue
		}

		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}

		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}

		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			remaining := strings.TrimPrefix(path, prefix+"/")
			if strings.HasPrefix(path, prefix+"/") && !strings.Contains(remaining, "/") {
				return true
			}
		}

		if strings.HasPrefix(path, pattern) {
			return true
		}
	}

	return false
}

func normalizeIncludePatterns(v any) []string {
	var out []string

	switch raw := v.(type) {
	case []string:
		out = raw
	case []any:
		for _, item := range raw {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				out = append(out, s)
			}
		}
	}

	return out
}

func parseGithubRepo(repoURL string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil {
		return "", "", err
	}

	if !strings.Contains(u.Host, "github.com") {
		return "", "", fmt.Errorf("only github.com repositories are supported")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository URL")
	}

	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func sanitizeChunkIDPart(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "doc"
	}

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "doc"
	}

	if len(out) > 80 {
		return out[:80]
	}

	return out
}

func stringPtr(s string) *string {
	return &s
}
