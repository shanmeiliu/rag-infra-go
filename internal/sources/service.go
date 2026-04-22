package sources

import (
	"context"
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
		if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, "notes", filename, extractedText, map[string]any{
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

	readmeContent, normalizedRepo, err := fetchGithubReadme(ctx, repoURL, branch)
	if err != nil {
		return nil, err
	}

	sourceKey := uuid.NewString()
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
			"branch":           branch,
			"include_patterns": includePatterns,
			"ingested_content": "README",
			"kind":             "github",
		},
		CreatedByUserID: createdBy,
	}

	id, err := s.repo.Create(ctx, src)
	if err != nil {
		return nil, err
	}
	src.ID = id

	if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, "repos", normalizedRepo, readmeContent, map[string]any{
		"repo_url":  repoURL,
		"repo_name": normalizedRepo,
		"branch":    branch,
		"kind":      "github",
	}); err != nil {
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

	switch kind {
	case "github":
		repoURL, _ := src.Metadata["repo_url"].(string)
		branch, _ := src.Metadata["branch"].(string)
		readmeContent, normalizedRepo, err := fetchGithubReadme(ctx, repoURL, branch)
		if err != nil {
			return nil, err
		}

		if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, "repos", normalizedRepo, readmeContent, map[string]any{
			"repo_url":  repoURL,
			"repo_name": normalizedRepo,
			"branch":    branch,
			"kind":      "github",
		}); err != nil {
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
			if err := s.ingestChunkedContent(ctx, src.SourceKey, src.SourceType, "notes", src.Name, extractedText, map[string]any{
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

func (s *Service) ingestChunkedContent(
	ctx context.Context,
	sourceID string,
	sourceType string,
	sourceGroup string,
	docName string,
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

	docID := "source-" + sourceID
	inputs := make([]ingestion.InputChunk, 0, len(chunks))

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("chunk-%s-%d", sourceID, i+1)
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

func fetchGithubReadme(ctx context.Context, repoURL, branch string) (string, string, error) {
	owner, repo, err := parseGithubRepo(repoURL)
	if err != nil {
		return "", "", err
	}
	if branch == "" {
		branch = "main"
	}

	candidates := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/README.md", owner, repo, branch),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/readme.md", owner, repo, branch),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/README.MD", owner, repo, branch),
	}

	client := &http.Client{Timeout: 15 * time.Second}

	for _, rawURL := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", "", err
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", "", readErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return string(body), owner + "/" + repo, nil
		}
	}

	return "", "", fmt.Errorf("could not fetch README from GitHub repo")
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

	return parts[0], parts[1], nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func stringPtr(s string) *string {
	return &s
}
