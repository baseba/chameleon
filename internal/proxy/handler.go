package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/yourusername/chameleon/internal/config"
	"github.com/yourusername/chameleon/internal/hash"
	"github.com/yourusername/chameleon/internal/storage"
)

// Handler implements the HTTP proxy handler
type Handler struct {
	config  *config.Config
	storage *storage.Storage
	proxy   *httputil.ReverseProxy
	logger  *log.Logger
}

// New creates a new proxy handler
func New(cfg *config.Config, st *storage.Storage, logger *log.Logger) (*Handler, error) {
	backendURL, err := url.Parse(cfg.BackendURL)
	if err != nil {
		return nil, fmt.Errorf("invalid backend URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	h := &Handler{
		config:  cfg,
		storage: st,
		proxy:   proxy,
		logger:  logger,
	}

	// Customize the proxy director
	originalDirector := proxy.Director
	// Capture logger and cfg for the closure
	loggerRef := logger
	mode := cfg.Mode
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = backendURL.Host

		// In record mode, strip conditional headers to force full responses
		// This prevents 304 (Not Modified) responses and ensures we get the actual resource
		if mode == config.ModeRecord {
			stripped := stripConditionalHeaders(req)
			if stripped {
				loggerRef.Printf("[RECORD] Stripped conditional headers to force full response")
			}
		}
	}

	return h, nil
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Read request body once (it will be consumed)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to read request body: %v", err)
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusInternalServerError)
		return
	}
	// Restore body for downstream use
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Generate hash from request
	requestHash, err := hash.Generate(r.Method, r.URL.Path, bytes.NewReader(bodyBytes))
	if err != nil {
		h.logger.Printf("[ERROR] Failed to generate hash: %v", err)
		http.Error(w, fmt.Sprintf("failed to generate hash: %v", err), http.StatusInternalServerError)
		return
	}

	// Log incoming request
	h.logger.Printf("[%s] %s %s | Hash: %s | Mode: %s",
		r.Method, r.URL.Path, r.RemoteAddr, requestHash[:16], h.config.Mode)

	switch h.config.Mode {
	case config.ModeReplay:
		h.handleReplay(w, r, requestHash, start)
	case config.ModeRecord:
		h.handleRecord(w, r, requestHash, bodyBytes, start)
	case config.ModePassthrough:
		h.handlePassthrough(w, r, start)
	default:
		h.logger.Printf("[ERROR] Unknown mode: %s", h.config.Mode)
		http.Error(w, fmt.Sprintf("unknown mode: %s", h.config.Mode), http.StatusInternalServerError)
	}
}

// handleReplay serves cached responses if available
func (h *Handler) handleReplay(w http.ResponseWriter, r *http.Request, requestHash string, start time.Time) {
	if !h.storage.Exists(requestHash) {
		h.logger.Printf("[REPLAY] No cached response found for hash: %s", requestHash)
		http.Error(w, fmt.Sprintf("no cached response found for request (hash: %s)", requestHash), http.StatusNotFound)
		return
	}

	cached, err := h.storage.Load(requestHash)
	if err != nil {
		h.logger.Printf("[REPLAY] Failed to load cached response: %v", err)
		http.Error(w, fmt.Sprintf("failed to load cached response: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Printf("[REPLAY] Serving cached response: %s %s | Status: %d | Hash: %s",
		cached.Method, cached.Path, cached.StatusCode, requestHash[:16])

	// Check if status code allows a response body
	// Status codes 1xx, 204 (No Content), and 304 (Not Modified) must not include a body
	statusAllowsBody := !(cached.StatusCode == 204 || cached.StatusCode == 304 || (cached.StatusCode >= 100 && cached.StatusCode < 200))

	// Set headers BEFORE WriteHeader (headers must be set before status code)
	// Use Set for the first value, Add for subsequent values to handle multi-value headers correctly
	for key, values := range cached.Headers {
		// Remove Content-Length header - we'll let Go calculate it automatically
		// This avoids conflicts when the actual body size differs
		if key == "Content-Length" {
			continue
		}
		if len(values) > 0 {
			w.Header().Set(key, values[0])
			for i := 1; i < len(values); i++ {
				w.Header().Add(key, values[i])
			}
		}
	}

	// Set status code
	w.WriteHeader(cached.StatusCode)

	// Only write body if status code allows it and body is not empty/null
	bodyStr := string(cached.Body)
	if statusAllowsBody && len(cached.Body) > 0 && bodyStr != "null" && bodyStr != "" {
		if _, err := w.Write(cached.Body); err != nil {
			h.logger.Printf("[ERROR] Failed to write response body: %v", err)
		}
	}

	duration := time.Since(start)
	h.logger.Printf("[REPLAY] Completed in %v", duration)
}

// handleRecord proxies to backend, captures response, saves to cache, and returns to client
func (h *Handler) handleRecord(w http.ResponseWriter, r *http.Request, requestHash string, bodyBytes []byte, start time.Time) {
	h.logger.Printf("[RECORD] Proxying to backend: %s", h.config.BackendURL)

	// Create a response writer that captures the response
	capturer := &responseCapturer{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default status code
		headers:        make(map[string][]string),
	}

	// Restore body for proxy
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Proxy the request
	h.proxy.ServeHTTP(capturer, r)

	// Capture response after proxying
	cached := &storage.CachedResponse{
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: capturer.statusCode,
		Headers:    capturer.headers,
		Body:       capturer.body,
	}

	// Save to cache
	if err := h.storage.Save(requestHash, cached); err != nil {
		h.logger.Printf("[ERROR] Failed to save cached response: %v", err)
	} else {
		h.logger.Printf("[RECORD] Saved response: %s %s | Status: %d | Hash: %s",
			cached.Method, cached.Path, cached.StatusCode, requestHash[:16])
	}

	duration := time.Since(start)
	h.logger.Printf("[RECORD] Completed in %v", duration)
}

// handlePassthrough just proxies without recording
func (h *Handler) handlePassthrough(w http.ResponseWriter, r *http.Request, start time.Time) {
	h.logger.Printf("[PASSTHROUGH] Proxying to backend: %s", h.config.BackendURL)
	h.proxy.ServeHTTP(w, r)
	duration := time.Since(start)
	h.logger.Printf("[PASSTHROUGH] Completed in %v", duration)
}

// responseCapturer captures the response for recording
type responseCapturer struct {
	http.ResponseWriter
	statusCode int
	headers    map[string][]string
	body       []byte
}

func (rc *responseCapturer) WriteHeader(code int) {
	rc.statusCode = code
	// Capture headers before writing them
	header := rc.ResponseWriter.Header()
	for key, values := range header {
		rc.headers[key] = make([]string, len(values))
		copy(rc.headers[key], values)
	}
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapturer) Write(b []byte) (int, error) {
	// Capture body
	rc.body = append(rc.body, b...)
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapturer) Header() http.Header {
	return rc.ResponseWriter.Header()
}

// stripConditionalHeaders removes HTTP conditional headers that can cause 304 responses
// This ensures we always get a full response (200) with the actual resource body in record mode
// Returns true if any headers were stripped
func stripConditionalHeaders(req *http.Request) bool {
	// List of headers that trigger conditional responses (304 Not Modified)
	conditionalHeaders := []string{
		"If-None-Match",       // ETag-based conditional request
		"If-Modified-Since",   // Date-based conditional request
		"If-Range",            // Range request conditional header
		"If-Match",            // ETag match for PUT/PATCH
		"If-Unmodified-Since", // Date-based conditional for PUT/PATCH
	}

	stripped := false
	for _, header := range conditionalHeaders {
		if req.Header.Get(header) != "" {
			req.Header.Del(header)
			stripped = true
		}
	}

	// Also remove Cache-Control header that might request validation
	// We want to get the actual content, not a cached/validated response
	cacheControl := req.Header.Get("Cache-Control")
	if cacheControl == "no-cache" || cacheControl == "max-age=0" {
		req.Header.Del("Cache-Control")
		stripped = true
	}

	return stripped
}

