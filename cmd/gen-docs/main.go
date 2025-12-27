package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/chameleon/internal/storage"
)

// RecordedRequest represents a recorded API request/response for documentation
type RecordedRequest struct {
	Hash       string
	Method     string
	Path       string
	StatusCode int
	Headers    map[string][]string
	Body       string
	BodyType   string // "json", "html", "text", "binary"
	Timestamp  time.Time
}

// DocsData holds all data for the HTML template
type DocsData struct {
	Title       string
	GeneratedAt string
	Requests    []RecordedRequest
	TotalCount  int
}

func main() {
	recordingsPath := "./recordings"
	outputPath := "./docs.html"

	// Allow override via command line
	if len(os.Args) > 1 {
		recordingsPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		outputPath = os.Args[2]
	}

	fmt.Printf("📚 Generating API documentation...\n")
	fmt.Printf("   Reading recordings from: %s\n", recordingsPath)
	fmt.Printf("   Output file: %s\n", outputPath)

	requests, err := loadAllRecordings(recordingsPath)
	if err != nil {
		log.Fatalf("Failed to load recordings: %v", err)
	}

	if len(requests) == 0 {
		log.Fatalf("No recordings found in %s", recordingsPath)
	}

	// Sort requests by method, then path
	sort.Slice(requests, func(i, j int) bool {
		if requests[i].Method != requests[j].Method {
			return requests[i].Method < requests[j].Method
		}
		return requests[i].Path < requests[j].Path
	})

	data := DocsData{
		Title:       "API Documentation",
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Requests:    requests,
		TotalCount:  len(requests),
	}

	if err := generateHTML(data, outputPath); err != nil {
		log.Fatalf("Failed to generate HTML: %v", err)
	}

	fmt.Printf("✅ Documentation generated successfully!\n")
	fmt.Printf("   Found %d recorded requests\n", len(requests))
	fmt.Printf("   Open %s in your browser to view\n", outputPath)
}

func loadAllRecordings(recordingsPath string) ([]RecordedRequest, error) {
	var requests []RecordedRequest

	entries, err := os.ReadDir(recordingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read recordings directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(recordingsPath, entry.Name())
		hash := strings.TrimSuffix(entry.Name(), ".json")

		req, err := loadRecording(filePath, hash)
		if err != nil {
			log.Printf("Warning: Failed to load %s: %v", entry.Name(), err)
			continue
		}

		requests = append(requests, req)
	}

	return requests, nil
}

func loadRecording(filePath, hash string) (RecordedRequest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return RecordedRequest{}, err
	}

	var cached storage.CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return RecordedRequest{}, err
	}

	// Get file modification time
	info, err := os.Stat(filePath)
	var timestamp time.Time
	if err == nil {
		timestamp = info.ModTime()
	}

	// Process body
	bodyStr, bodyType := formatBody(cached.Body)

	return RecordedRequest{
		Hash:       hash,
		Method:     cached.Method,
		Path:       cached.Path,
		StatusCode: cached.StatusCode,
		Headers:    cached.Headers,
		Body:       bodyStr,
		BodyType:   bodyType,
		Timestamp:  timestamp,
	}, nil
}

func formatBody(body storage.ResponseBody) (string, string) {
	if len(body) == 0 {
		return "", "empty"
	}

	// Try to parse as JSON first
	var jsonValue interface{}
	if err := json.Unmarshal(body, &jsonValue); err == nil {
		// It's valid JSON, pretty print it
		prettyJSON, err := json.MarshalIndent(jsonValue, "", "  ")
		if err == nil {
			return string(prettyJSON), "json"
		}
	}

	// Check if it's HTML
	bodyStr := string(body)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
		return bodyStr, "html"
	}

	// Check if it's base64 encoded
	if decoded, err := base64.StdEncoding.DecodeString(bodyStr); err == nil && len(decoded) > 0 {
		decodedStr := string(decoded)
		// Try to detect decoded content type
		if strings.HasPrefix(strings.TrimSpace(decodedStr), "<") {
			return decodedStr, "html"
		}
		// Check if decoded is JSON
		if json.Unmarshal(decoded, &jsonValue) == nil {
			prettyJSON, err := json.MarshalIndent(jsonValue, "", "  ")
			if err == nil {
				return string(prettyJSON), "json"
			}
		}
		return decodedStr, "text"
	}

	// Default to text
	return bodyStr, "text"
}

func generateHTML(data DocsData, outputPath string) error {
	funcMap := template.FuncMap{
		"lower":      strings.ToLower,
		"statusClass": statusClass,
	}

	tmpl, err := template.New("docs").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func statusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "other"
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Chameleon</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #f5f5f5;
            color: #333;
            line-height: 1.6;
        }

        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 2rem;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }

        .header h1 {
            font-size: 2rem;
            margin-bottom: 0.5rem;
        }

        .header p {
            opacity: 0.9;
            font-size: 0.9rem;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }

        .stats {
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            display: flex;
            gap: 2rem;
            flex-wrap: wrap;
        }

        .stat-item {
            display: flex;
            flex-direction: column;
        }

        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: #667eea;
        }

        .stat-label {
            font-size: 0.9rem;
            color: #666;
            margin-top: 0.25rem;
        }

        .filters {
            background: white;
            padding: 1rem;
            border-radius: 8px;
            margin-bottom: 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            display: flex;
            gap: 1rem;
            flex-wrap: wrap;
            align-items: center;
        }

        .filter-group {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .filter-group label {
            font-weight: 500;
            color: #666;
        }

        .filter-group input,
        .filter-group select {
            padding: 0.5rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 0.9rem;
        }

        .request-card {
            background: white;
            border-radius: 8px;
            margin-bottom: 1.5rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
            transition: box-shadow 0.2s;
        }

        .request-card:hover {
            box-shadow: 0 4px 8px rgba(0,0,0,0.15);
        }

        .request-header {
            padding: 1.5rem;
            border-bottom: 1px solid #eee;
            cursor: pointer;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .request-header:hover {
            background: #f9f9f9;
        }

        .request-method {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-weight: bold;
            font-size: 0.85rem;
            margin-right: 1rem;
            text-transform: uppercase;
        }

        .method-get { background: #e3f2fd; color: #1976d2; }
        .method-post { background: #e8f5e9; color: #388e3c; }
        .method-put { background: #fff3e0; color: #f57c00; }
        .method-patch { background: #fce4ec; color: #c2185b; }
        .method-delete { background: #ffebee; color: #d32f2f; }
        .method-options { background: #f3e5f5; color: #7b1fa2; }

        .request-path {
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 1rem;
            color: #333;
            flex: 1;
        }

        .request-status {
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-weight: bold;
            font-size: 0.85rem;
        }

        .status-2xx { background: #e8f5e9; color: #2e7d32; }
        .status-3xx { background: #fff3e0; color: #f57c00; }
        .status-4xx { background: #ffebee; color: #c62828; }
        .status-5xx { background: #ffebee; color: #d32f2f; }

        .request-toggle {
            margin-left: 1rem;
            color: #999;
            font-size: 0.9rem;
        }

        .request-content {
            display: none;
            padding: 1.5rem;
        }

        .request-content.active {
            display: block;
        }

        .section {
            margin-bottom: 2rem;
        }

        .section-title {
            font-size: 1.1rem;
            font-weight: 600;
            margin-bottom: 1rem;
            color: #667eea;
            padding-bottom: 0.5rem;
            border-bottom: 2px solid #667eea;
        }

        .headers-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 0.5rem;
        }

        .headers-table th,
        .headers-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid #eee;
        }

        .headers-table th {
            background: #f9f9f9;
            font-weight: 600;
            color: #666;
        }

        .headers-table td {
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.9rem;
        }

        .body-container {
            background: #f9f9f9;
            border: 1px solid #ddd;
            border-radius: 4px;
            padding: 1rem;
            overflow-x: auto;
        }

        .body-content {
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.9rem;
            white-space: pre-wrap;
            word-wrap: break-word;
        }

        .body-json {
            color: #333;
        }

        .body-html {
            color: #0066cc;
        }

        .body-text {
            color: #333;
        }

        .no-results {
            text-align: center;
            padding: 3rem;
            color: #999;
        }

        .hash {
            font-size: 0.8rem;
            color: #999;
            font-family: 'Monaco', 'Menlo', monospace;
            margin-top: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🦎 Chameleon API Documentation</h1>
        <p>Generated on {{.GeneratedAt}} • {{.TotalCount}} recorded requests</p>
    </div>

    <div class="container">
        <div class="stats">
            <div class="stat-item">
                <div class="stat-value">{{.TotalCount}}</div>
                <div class="stat-label">Total Requests</div>
            </div>
            <div class="stat-item">
                <div class="stat-value" id="visible-count">{{.TotalCount}}</div>
                <div class="stat-label">Visible</div>
            </div>
        </div>

        <div class="filters">
            <div class="filter-group">
                <label for="search">Search:</label>
                <input type="text" id="search" placeholder="Filter by path, method, or status..." style="min-width: 300px;">
            </div>
            <div class="filter-group">
                <label for="method-filter">Method:</label>
                <select id="method-filter">
                    <option value="">All Methods</option>
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                    <option value="PUT">PUT</option>
                    <option value="PATCH">PATCH</option>
                    <option value="DELETE">DELETE</option>
                    <option value="OPTIONS">OPTIONS</option>
                </select>
            </div>
            <div class="filter-group">
                <label for="status-filter">Status:</label>
                <select id="status-filter">
                    <option value="">All Statuses</option>
                    <option value="2xx">2xx Success</option>
                    <option value="3xx">3xx Redirect</option>
                    <option value="4xx">4xx Client Error</option>
                    <option value="5xx">5xx Server Error</option>
                </select>
            </div>
        </div>

        <div id="requests-container">
            {{range .Requests}}
            <div class="request-card" data-method="{{.Method}}" data-path="{{.Path}}" data-status="{{.StatusCode}}">
                <div class="request-header" onclick="toggleRequest('{{.Hash}}')">
                    <div style="display: flex; align-items: center; flex: 1;">
                        <span class="request-method method-{{.Method | lower}}">{{.Method}}</span>
                        <span class="request-path">{{.Path}}</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 1rem;">
                        <span class="request-status status-{{statusClass .StatusCode}}">{{.StatusCode}}</span>
                        <span class="request-toggle" id="toggle-{{.Hash}}">▼</span>
                    </div>
                </div>
                <div class="request-content" id="content-{{.Hash}}">
                    <div class="hash">Hash: {{.Hash}}</div>

                    <div class="section">
                        <div class="section-title">Response Headers</div>
                        <table class="headers-table">
                            <thead>
                                <tr>
                                    <th>Header</th>
                                    <th>Value</th>
                                </tr>
                            </thead>
                            <tbody>
                                {{range $key, $values := .Headers}}
                                <tr>
                                    <td><strong>{{$key}}</strong></td>
                                    <td>{{range $values}}{{.}}<br>{{end}}</td>
                                </tr>
                                {{end}}
                            </tbody>
                        </table>
                    </div>

                    <div class="section">
                        <div class="section-title">Response Body</div>
                        <div class="body-container">
                            <div class="body-content body-{{.BodyType}}">{{.Body | html}}</div>
                        </div>
                    </div>
                </div>
            </div>
            {{end}}
        </div>

        <div class="no-results" id="no-results" style="display: none;">
            <p>No requests match your filters.</p>
        </div>
    </div>

    <script>
        function toggleRequest(hash) {
            const content = document.getElementById('content-' + hash);
            const toggle = document.getElementById('toggle-' + hash);

            if (content.classList.contains('active')) {
                content.classList.remove('active');
                toggle.textContent = '▼';
            } else {
                content.classList.add('active');
                toggle.textContent = '▲';
            }
        }

        function updateVisibleCount() {
            const visible = document.querySelectorAll('.request-card[style*="display: block"], .request-card:not([style*="display: none"])');
            const visibleCount = Array.from(visible).filter(card =>
                !card.style.display || card.style.display !== 'none'
            ).length;
            document.getElementById('visible-count').textContent = visibleCount;
        }

        function filterRequests() {
            const search = document.getElementById('search').value.toLowerCase();
            const methodFilter = document.getElementById('method-filter').value;
            const statusFilter = document.getElementById('status-filter').value;

            const cards = document.querySelectorAll('.request-card');
            let visibleCount = 0;

            cards.forEach(card => {
                const method = card.dataset.method;
                const path = card.dataset.path.toLowerCase();
                const status = parseInt(card.dataset.status);

                // Search filter
                const matchesSearch = !search ||
                    path.includes(search) ||
                    method.toLowerCase().includes(search) ||
                    status.toString().includes(search);

                // Method filter
                const matchesMethod = !methodFilter || method === methodFilter;

                // Status filter
                let matchesStatus = true;
                if (statusFilter) {
                    const statusPrefix = Math.floor(status / 100);
                    matchesStatus =
                        (statusFilter === '2xx' && statusPrefix === 2) ||
                        (statusFilter === '3xx' && statusPrefix === 3) ||
                        (statusFilter === '4xx' && statusPrefix === 4) ||
                        (statusFilter === '5xx' && statusPrefix === 5);
                }

                if (matchesSearch && matchesMethod && matchesStatus) {
                    card.style.display = 'block';
                    visibleCount++;
                } else {
                    card.style.display = 'none';
                }
            });

            document.getElementById('visible-count').textContent = visibleCount;
            document.getElementById('no-results').style.display = visibleCount === 0 ? 'block' : 'none';
        }

        document.getElementById('search').addEventListener('input', filterRequests);
        document.getElementById('method-filter').addEventListener('change', filterRequests);
        document.getElementById('status-filter').addEventListener('change', filterRequests);
    </script>
</body>
</html>
`

