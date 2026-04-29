// Package proxy implements an HTTP/HTTPS proxy server that intercepts
// WeChat Channels (视频号) video requests to extract download URLs.
package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// VideoInfo holds extracted information about a WeChat Channels video.
type VideoInfo struct {
	URL      string
	FileID   string
	Quality  string
	Headers  map[string]string
}

// Handler is the proxy handler that intercepts video requests.
type Handler struct {
	mu      sync.Mutex
	videos  []VideoInfo
	onVideo func(VideoInfo)
}

// NewHandler creates a new proxy handler with an optional callback
// that is invoked whenever a new video URL is detected.
func NewHandler(onVideo func(VideoInfo)) *Handler {
	return &Handler{
		videos:  make([]VideoInfo, 0),
		onVideo: onVideo,
	}
}

// ServeHTTP implements http.Handler and proxies requests while
// inspecting responses for WeChat Channels video content.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CONNECT method for HTTPS tunneling
	if r.Method == http.MethodConnect {
		h.handleTunnel(w, r)
		return
	}

	// Check if this is a video request
	if h.isVideoRequest(r) {
		h.captureVideoInfo(r)
	}

	// Forward the request
	target, err := url.Parse(fmt.Sprintf("%s://%s", schemeFor(r), r.Host))
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxy.ServeHTTP(w, r)
}

// handleTunnel establishes a TCP tunnel for HTTPS connections.
func (h *Handler) handleTunnel(w http.ResponseWriter, r *http.Request) {
	dest, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("[proxy] hijacking not supported")
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("[proxy] hijack error: %v", err)
		return
	}

	go transfer(dest, clientConn)
	go transfer(clientConn, dest)
}

// isVideoRequest returns true if the request appears to be for a
// WeChat Channels video file.
func (h *Handler) isVideoRequest(r *http.Request) bool {
	host := r.Host
	path := r.URL.Path
	return (strings.Contains(host, "finder.video.qq.com") ||
		strings.Contains(host, "channels.weixin.qq.com")) &&
		(strings.Contains(path, ".mp4") || strings.Contains(path, "filekey"))
}

// captureVideoInfo extracts and stores video metadata from the request.
func (h *Handler) captureVideoInfo(r *http.Request) {
	info := VideoInfo{
		URL:     fmt.Sprintf("https://%s%s", r.Host, r.URL.RequestURI()),
		FileID:  extractFileID(r.URL),
		Quality: extractQuality(r.URL),
		Headers: map[string]string{
			"User-Agent": r.Header.Get("User-Agent"),
			"Referer":    r.Header.Get("Referer"),
		},
	}

	h.mu.Lock()
	h.videos = append(h.videos, info)
	h.mu.Unlock()

	log.Printf("[proxy] captured video: fileID=%s quality=%s", info.FileID, info.Quality)

	if h.onVideo != nil {
		h.onVideo(info)
	}
}

// GetVideos returns a snapshot of all captured video infos.
func (h *Handler) GetVideos() []VideoInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]VideoInfo, len(h.videos))
	copy(result, h.videos)
	return result
}

// extractFileID attempts to parse the file identifier from the URL.
func extractFileID(u *url.URL) string {
	if fid := u.Query().Get("filekey"); fid != "" {
		return fid
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

// extractQuality attempts to determine video quality from URL parameters.
func extractQuality(u *url.URL) string {
	if q := u.Query().Get("quality"); q != "" {
		return q
	}
	path := u.Path
	switch {
	case strings.Contains(path, "1080"):
		return "1080p"
	case strings.Contains(path, "720"):
		return "720p"
	case strings.Contains(path, "540"):
		return "540p"
	default:
		return "unknown"
	}
}

// schemeFor returns "https" if the request was made over TLS, else "http".
func schemeFor(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
