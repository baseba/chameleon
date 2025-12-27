# Chameleon 🦎

An HTTP recording proxy server that sits between a frontend and backend, capturing and replaying API responses. Perfect for offline development, testing, and debugging.

## Features

- **Record Mode**: Captures API responses from your backend and saves them for later use
- **Replay Mode**: Serves cached responses without hitting the backend
- **Passthrough Mode**: Proxies requests without recording (for development)
- **Smart Caching**: Uses SHA256 hashing based on method, path, and body for cache keys
- **Pretty JSON Storage**: Human-readable cached responses stored as JSON files

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/chameleon.git
cd chameleon
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o chameleon ./cmd/chameleon
```

## Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Available configuration options:

| Variable | Description | Default |
|----------|-------------|---------|
| `MODE` | Operation mode: `record`, `replay`, or `passthrough` | `record` |
| `BACKEND_URL` | Backend server URL to proxy to | `http://localhost:8080` |
| `PORT` | Port for the proxy server | `3000` |
| `STORAGE_PATH` | Directory to store cached responses | `./recordings` |

## Usage

### Quick Start with Command-Line Arguments

You can specify the port and backend URL directly when running chameleon:

```bash
# Run on port 3000, proxying to api.example.com
./chameleon 3000 api.example.com

# Just specify the port (backend uses default or environment variable)
./chameleon 3000

# Use default values (port 3000, backend http://localhost:8080)
./chameleon
```

**Command-line format:**
```bash
./chameleon [port] [backend]
```

- `port`: Port number for the proxy server (optional, default: 3000)
- `backend`: Backend URL or hostname (optional, default: http://localhost:8080)
  - If no scheme is provided, `http://` is assumed
  - Examples: `api.example.com`, `https://api.example.com`, `localhost:8080`

**Note:** Command-line arguments take precedence over environment variables.

### Record Mode

Capture API responses from your backend:

```bash
# Using command-line arguments
MODE=record ./chameleon 3000 api.example.com

# Or using environment variables
MODE=record BACKEND_URL=http://localhost:8080 PORT=3000 ./chameleon
```

**Flow:**
1. Frontend sends request to `http://localhost:3000/api/users`
2. Chameleon strips conditional headers (If-None-Match, If-Modified-Since, etc.) to force full responses
3. Chameleon proxies request to backend
4. Backend responds with full resource (200 OK) and JSON body
5. Chameleon saves response to `recordings/<hash>.json`
6. Response is forwarded to frontend

**Note:** In record mode, Chameleon automatically removes conditional headers like `If-None-Match` and `If-Modified-Since` to prevent 304 (Not Modified) responses. This ensures you always capture the full resource content, not just validation responses.

### Replay Mode

Serve cached responses without hitting the backend:

```bash
# Using command-line arguments
MODE=replay ./chameleon 3000

# Or using environment variables
MODE=replay PORT=3000 ./chameleon
```

**Flow:**
1. Frontend sends request to `http://localhost:3000/api/users`
2. Chameleon generates hash from request
3. Chameleon loads `recordings/<hash>.json`
4. Cached response is served to frontend
5. Returns 404 if no cached response exists

### Passthrough Mode

Proxy requests without recording:

```bash
# Using command-line arguments
MODE=passthrough ./chameleon 3000 api.example.com

# Or using environment variables
MODE=passthrough BACKEND_URL=http://localhost:8080 PORT=3000 ./chameleon
```

**Flow:**
1. Frontend sends request to proxy
2. Chameleon forwards request to backend
3. Response is returned without caching

## Example

1. Start your backend server on port 8080
2. Start Chameleon in record mode:
```bash
# Using command-line arguments (recommended)
MODE=record ./chameleon 3000 localhost:8080

# Or using environment variables
MODE=record BACKEND_URL=http://localhost:8080 PORT=3000 ./chameleon
```

3. Make requests through the proxy:
```bash
curl http://localhost:3000/api/users
```

4. Check the recordings directory:
```bash
ls recordings/
# You'll see JSON files like: a1b2c3d4e5f6...json
```

5. Stop your backend and switch to replay mode:
```bash
MODE=replay PORT=3000 ./chameleon
```

6. Make the same request again - it will work even though the backend is down:
```bash
curl http://localhost:3000/api/users
```

## How It Works

Chameleon generates a unique hash for each request based on:
- HTTP method (GET, POST, etc.)
- URL path
- Request body (if present)

This hash is used as the filename for cached responses, ensuring that requests with the same method, path, and body will be served the same cached response.

Example hash generation:
- Method: `POST`
- Path: `/api/users`
- Body: `{"name": "John"}`
- Hash: `a1b2c3d4e5f6...` (SHA256 hex)

## Project Structure

```
chameleon/
├── cmd/
│   ├── chameleon/
│   │   └── main.go          # Application entry point
│   └── gen-docs/
│       └── main.go          # Documentation generator
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── proxy/
│   │   └── handler.go       # HTTP proxy handler
│   ├── storage/
│   │   └── storage.go       # Cache storage operations
│   └── hash/
│       └── hash.go          # Request hashing
├── recordings/              # Cached responses (gitignored)
├── go.mod
├── go.sum
├── .gitignore
├── .env.example
└── README.md
```

## Roadmap

Future features planned:

- [x] Web UI for viewing and managing cached responses (documentation generator)
- [ ] Request/response filtering and transformation
- [ ] Response modification (delay simulation, error injection)
- [ ] Cache expiration and cleanup
- [ ] Support for streaming responses
- [ ] Metrics and monitoring
- [ ] Request matching rules (custom hashing strategies)
- [ ] Export/import cache functionality

## Generating API Documentation

Chameleon includes a documentation generator that creates an interactive HTML page from all recorded requests:

```bash
# Generate docs from recordings
go run ./cmd/gen-docs ./recordings docs.html

# Or build the generator first
go build -o gen-docs ./cmd/gen-docs
./gen-docs ./recordings docs.html
```

The generated HTML file includes:
- **Interactive interface** with search and filtering
- **All recorded requests** with method, path, status code
- **Response headers** in a readable table format
- **Response bodies** with syntax highlighting for JSON
- **Status code colors** for easy identification
- **Expandable/collapsible** request details

Open the generated `docs.html` file in your browser to view your API documentation.

## Development

Run tests:
```bash
go test ./...
```

Build for different platforms:
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o chameleon-linux ./cmd/chameleon

# macOS
GOOS=darwin GOARCH=amd64 go build -o chameleon-macos ./cmd/chameleon

# Windows
GOOS=windows GOARCH=amd64 go build -o chameleon.exe ./cmd/chameleon
```

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
