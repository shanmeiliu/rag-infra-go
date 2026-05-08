package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/shanmeiliu/rag-infra-go/internal/catprofile"
)

type CatProfileHandler struct {
	repo     *catprofile.Repository
	photoDir string
}

func NewCatProfileHandler(repo *catprofile.Repository, photoDir string) *CatProfileHandler {
	if photoDir == "" {
		photoDir = "./uploads/cat-profile"
	}

	return &CatProfileHandler{
		repo:     repo,
		photoDir: photoDir,
	}
}

func (h *CatProfileHandler) PublicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profile, err := h.repo.GetProfile(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stories, err := h.repo.ListStories(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	photos, err := h.repo.ListPhotos(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"profile": profile,
		"stories": stories,
		"photos":  photos,
	})
}

func (h *CatProfileHandler) AdminProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.adminGetProfile(w, r)
	case http.MethodPut:
		h.adminUpdateProfile(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CatProfileHandler) adminGetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.repo.GetProfile(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stories, err := h.repo.ListStories(r.Context(), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	photos, err := h.repo.ListPhotos(r.Context(), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"profile": profile,
		"stories": stories,
		"photos":  photos,
	})
}

func (h *CatProfileHandler) adminUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName   string `json:"display_name"`
		Tagline       string `json:"tagline"`
		Bio           string `json:"bio"`
		AvatarPhotoID *int64 `json:"avatar_photo_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	profile, err := h.repo.UpdateProfile(
		r.Context(),
		strings.TrimSpace(req.DisplayName),
		strings.TrimSpace(req.Tagline),
		strings.TrimSpace(req.Bio),
		req.AvatarPhotoID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"profile": profile})
}

func (h *CatProfileHandler) AdminStories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stories, err := h.repo.ListStories(r.Context(), true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"stories": stories})

	case http.MethodPost:
		var req catprofile.Story
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		story, err := h.repo.CreateStory(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"story": story})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CatProfileHandler) AdminStoryByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDFromPath(r.URL.Path, "/api/admin/cat-profile/stories/")
	if !ok {
		http.Error(w, "invalid story id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req catprofile.Story
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		story, err := h.repo.UpdateStory(r.Context(), id, req)
		if err != nil {
			writeCatProfileError(w, err)
			return
		}

		writeJSON(w, map[string]any{"story": story})

	case http.MethodDelete:
		if err := h.repo.DeleteStory(r.Context(), id); err != nil {
			writeCatProfileError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CatProfileHandler) AdminPhotos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		photos, err := h.repo.ListPhotos(r.Context(), true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"photos": photos})

	case http.MethodPost:
		h.uploadPhoto(w, r)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CatProfileHandler) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(h.photoDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !isAllowedImageContentType(contentType) {
		http.Error(w, "unsupported image type", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = extensionFromContentType(contentType)
	}

	filename := uuid.NewString() + ext
	targetPath := filepath.Join(h.photoDir, filename)

	out, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	isPublished := parseBoolDefault(r.FormValue("is_published"), true)

	photo := catprofile.Photo{
		Filename:         filename,
		OriginalFilename: header.Filename,
		ContentType:      contentType,
		FilePath:         targetPath,
		PublicURL:        "/api/cat-profile/photos/" + filename,
		Caption:          r.FormValue("caption"),
		AltText:          r.FormValue("alt_text"),
		SortOrder:        sortOrder,
		IsPublished:      isPublished,
	}

	created, err := h.repo.CreatePhoto(r.Context(), photo)
	if err != nil {
		_ = os.Remove(targetPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"photo": created})
}

func (h *CatProfileHandler) AdminPhotoByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDFromPath(r.URL.Path, "/api/admin/cat-profile/photos/")
	if !ok {
		http.Error(w, "invalid photo id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req catprofile.Photo
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		photo, err := h.repo.UpdatePhoto(r.Context(), id, req)
		if err != nil {
			writeCatProfileError(w, err)
			return
		}

		writeJSON(w, map[string]any{"photo": photo})

	case http.MethodDelete:
		photo, err := h.repo.DeletePhoto(r.Context(), id)
		if err != nil {
			writeCatProfileError(w, err)
			return
		}

		_ = os.Remove(photo.FilePath)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CatProfileHandler) ServePhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/api/cat-profile/photos/")
	filename = filepath.Base(filename)
	if filename == "" || filename == "." {
		http.NotFound(w, r)
		return
	}

	photo, err := h.repo.GetPhotoByFilename(r.Context(), filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !photo.IsPublished {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", photo.ContentType)
	http.ServeFile(w, r, photo.FilePath)
}

func writeCatProfileError(w http.ResponseWriter, err error) {
	if errors.Is(err, catprofile.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func parseIDFromPath(path, prefix string) (int64, bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}

	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

func isAllowedImageContentType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func extensionFromContentType(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".img"
	}
}

func parseBoolDefault(raw string, fallback bool) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return fallback
	}

	switch raw {
	case "1", "true", "yes", "y":
		return true
	case "0", "false", "no", "n":
		return false
	default:
		return fallback
	}
}

func _unusedFmt() {
	_ = fmt.Sprintf
}
