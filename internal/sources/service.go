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

type Service struct {
	repo         *Repository
	ingestionSvc IngestionService
	uploadDir    string
}

func NewService(repo *Repository, ingestionSvc IngestionService, uploadDir string) *Service {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &Service{
		repo:         repo,
		ingestionSvc: ingestionSvc,
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
			"filename": filename,
			"size":     len(contentBytes),
		},
		CreatedByUserID: createdBy,
	}

	id, err := s.repo.Create(ctx, src)
	if err != nil {
		return nil, err
	}
	src.ID = id

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".txt" || ext == ".md" {
		docID := "source-" + sourceKey
		chunkID := "chunk-" + sourceKey + "-1"

		err = s.ingestionSvc.Ingest(ctx, []ingestion.InputChunk{
			{
				ChunkID: chunkID,
				DocID:   docID,
				Content: string(contentBytes),
				Metadata: map[string]any{
					"source_group": "notes",
					"source_type":  sourceType,
					"source_id":    sourceKey,
					"filename":     filename,
				},
			},
		})
		if err != nil {
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
		SourceKey:       sourceKey,
		Name:            normalizedRepo,
		SourceType:      sourceType,
		Status:          "ready",
		Origin:          &repoURL,
		CreatedByUserID: createdBy,
		Metadata: map[string]any{
			"repo_url":         repoURL,
			"normalized_repo":  normalizedRepo,
			"branch":           branch,
			"include_patterns": includePatterns,
			"ingested_content": "README",
		},
	}

	id, err := s.repo.Create(ctx, src)
	if err != nil {
		return nil, err
	}
	src.ID = id

	docID := "source-" + sourceKey
	chunkID := "chunk-" + sourceKey + "-1"

	err = s.ingestionSvc.Ingest(ctx, []ingestion.InputChunk{
		{
			ChunkID: chunkID,
			DocID:   docID,
			Content: readmeContent,
			Metadata: map[string]any{
				"source_group": "repos",
				"source_type":  sourceType,
				"source_id":    sourceKey,
				"repo_url":     repoURL,
				"repo_name":    normalizedRepo,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return src, nil
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
