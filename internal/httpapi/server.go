package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/1996fanrui/gh-downloader/internal/app"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	service *app.Service
	client  *http.Client
	assets  http.Handler
}

func NewServer(service *app.Service, client *http.Client) *Server {
	assetFS, err := fs.Sub(staticFiles, "static/assets")
	if err != nil {
		panic(err)
	}
	return &Server{service: service, client: client, assets: http.FileServer(http.FS(assetFS))}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("POST /api/repos", s.addRepo)
	mux.HandleFunc("/", s.route)
	return mux
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/index.html" || strings.HasPrefix(r.URL.Path, "/repos/")):
		s.index(w)
	case r.Method == http.MethodGet && r.URL.Path == "/favicon.ico":
		s.favicon(w)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/assets/"):
		s.asset(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/repos/"):
		s.getRepo(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/repos/") && strings.HasSuffix(r.URL.Path, "/sync"):
		s.syncRepo(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/dl/"):
		s.download(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) index(w http.ResponseWriter) {
	indexHTML, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "index file not found")
		return
	}
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

func (s *Server) favicon(w http.ResponseWriter) {
	icon, err := staticFiles.ReadFile("static/assets/icon.png")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "icon file not found")
		return
	}
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icon)
}

func (s *Server) asset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	http.StripPrefix("/assets/", s.assets).ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.service.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (s *Server) addRepo(w http.ResponseWriter, r *http.Request) {
	var req addRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repoSpec := req.Repo
	if repoSpec == "" {
		repoSpec = req.URL
	}
	repo, err := s.service.AddRepo(r.Context(), repoSpec)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	owner, repo, ok := repoPath(strings.TrimPrefix(r.URL.Path, "/api/repos/"))
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	item, found, err := s.service.GetRepo(r.Context(), owner, repo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) syncRepo(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/repos/"), "/sync")
	owner, repo, ok := repoPath(path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	item, err := s.service.SyncRepo(r.Context(), owner, repo)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	owner, repo, tag, assetName, ok := downloadPath(strings.TrimPrefix(r.URL.Path, "/dl/"))
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	asset, found, err := s.service.ResolveAsset(r.Context(), owner, repo, tag, assetName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err := s.proxyAsset(r.Context(), w, r, asset.DownloadURL, asset.Name); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
	}
}

func (s *Server) proxyAsset(ctx context.Context, w http.ResponseWriter, r *http.Request, upstream string, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream, nil)
	if err != nil {
		return err
	}
	if value := r.Header.Get("Range"); value != "" {
		req.Header.Set("Range", value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch upstream asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upstream returned %s: %s", resp.Status, string(body))
	}

	copyHeader(w.Header(), resp.Header, "Content-Type", "Content-Length", "Accept-Ranges", "Content-Range", "Last-Modified", "ETag")
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

func repoPath(path string) (string, string, bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", false
	}
	return owner, repo, owner != "" && repo != ""
}

func downloadPath(path string) (string, string, string, string, bool) {
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		return "", "", "", "", false
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", "", "", false
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", "", "", false
	}
	tag, err := url.PathUnescape(parts[2])
	if err != nil {
		return "", "", "", "", false
	}
	assetName, err := url.PathUnescape(strings.Join(parts[3:], "/"))
	if err != nil {
		return "", "", "", "", false
	}
	return owner, repo, tag, assetName, owner != "" && repo != "" && tag != "" && assetName != ""
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func copyHeader(target http.Header, source http.Header, keys ...string) {
	for _, key := range keys {
		if value := source.Get(key); value != "" {
			target.Set(key, value)
		}
	}
}

func contentDisposition(filename string) string {
	return fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename))
}

type addRepoRequest struct {
	Repo string `json:"repo"`
	URL  string `json:"url"`
}

type errorResponse struct {
	Error string `json:"error"`
}
