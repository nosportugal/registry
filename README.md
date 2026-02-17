# MCP Registry

The MCP registry provides MCP clients with a read-only list of MCP servers, like an app store for MCP servers.

[**⚡️ Live API docs**](https://registry.modelcontextprotocol.io/docs) | [**👀 Ecosystem vision**](docs/design/ecosystem-vision.md) | 📖 **[Full documentation](./docs)**

## Overview

This is a **read-only** MCP registry that serves server listings from a seed file. All server registration is managed externally (e.g., via GCP Apigee) and loaded via a seed file on startup.

### Key Features

- **Read-Only API**: Provides GET endpoints for discovering MCP servers
- **Seed-Based**: All server data loaded from a seed file (supports local files or HTTP URLs)
- **Full Reload**: Database is cleared and reloaded on each startup
- **Validation**: Built-in server.json validation
- **API Versioning**: Supports v0 and v0.1 API versions

## Development Status

**2025-10-24 update**: The Registry API has entered an **API freeze (v0.1)** 🎉. For the next month or more, the API will remain stable with no breaking changes, allowing integrators to confidently implement support. This freeze applies to v0.1 while development continues on v0. We'll use this period to validate the API in real-world integrations and gather feedback to shape v1 for general availability. Thank you to everyone for your contributions and patience—your involvement has been key to getting us here!

**2025-09-08 update**: The registry has launched in preview 🎉 ([announcement blog post](https://blog.modelcontextprotocol.io/posts/2025-09-08-mcp-registry-preview/)). While the system is now more stable, this is still a preview release and breaking changes or data resets may occur. A general availability (GA) release will follow later. We'd love your feedback in [GitHub discussions](https://github.com/modelcontextprotocol/registry/discussions/new?category=ideas) or in the [#registry-dev Discord](https://discord.com/channels/1358869848138059966/1369487942862504016) ([joining details here](https://modelcontextprotocol.io/community/communication)).

Current key maintainers:
- **Adam Jones** (Anthropic) [@domdomegg](https://github.com/domdomegg)  
- **Tadas Antanavicius** (PulseMCP) [@tadasant](https://github.com/tadasant)
- **Toby Padilla** (GitHub) [@toby](https://github.com/toby)
- **Radoslav (Rado) Dimitrov** (Stacklok) [@rdimitrov](https://github.com/rdimitrov)

## Contributing

We use multiple channels for collaboration - see [modelcontextprotocol.io/community/communication](https://modelcontextprotocol.io/community/communication).

Often (but not always) ideas flow through this pipeline:

- **[Discord](https://modelcontextprotocol.io/community/communication)** - Real-time community discussions
- **[Discussions](https://github.com/modelcontextprotocol/registry/discussions)** - Propose and discuss product/technical requirements
- **[Issues](https://github.com/modelcontextprotocol/registry/issues)** - Track well-scoped technical work  
- **[Pull Requests](https://github.com/modelcontextprotocol/registry/pulls)** - Contribute work towards issues

### Quick start:

#### Pre-requisites

- **Docker**
- **Go 1.24.x**
- **ko** - Container image builder for Go ([installation instructions](https://ko.build/install/))
- **golangci-lint v2.4.0**

#### Running the server

```bash
# Start full development environment
make dev-compose
```

This starts the registry at [`localhost:8080`](http://localhost:8080) with PostgreSQL. The database uses ephemeral storage and is reset each time you restart the containers, ensuring a clean state for development and testing.

The registry loads MCP servers from `data/seed.json` on startup. You can modify this file to test different server configurations.

The setup can be configured with environment variables in [docker-compose.yml](./docker-compose.yml) - see [.env.example](./.env.example) for a reference.

<details>
<summary>Alternative: Running a pre-built Docker image</summary>

Pre-built Docker images are automatically published to GitHub Container Registry:

```bash
# Run latest stable release
docker run -p 8080:8080 ghcr.io/modelcontextprotocol/registry:latest

# Run latest from main branch (continuous deployment)
docker run -p 8080:8080 ghcr.io/modelcontextprotocol/registry:main

# Run specific release version
docker run -p 8080:8080 ghcr.io/modelcontextprotocol/registry:v1.0.0

# Run development build from main branch
docker run -p 8080:8080 ghcr.io/modelcontextprotocol/registry:main-20250906-abc123d
```

**Available tags:** 
- **Releases**: `latest`, `v1.0.0`, `v1.1.0`, etc.
- **Continuous**: `main` (latest main branch build)
- **Development**: `main-<date>-<sha>` (specific commit builds)

</details>

### Seed File Format

The registry expects a JSON array of ServerJSON objects in `data/seed.json`:

```json
[
  {
    "name": "io.example/my-server",
    "version": "1.0.0",
    "description": "My MCP server",
    "remotes": [
      {
        "type": "stdio",
        "url": "npx -y @example/my-server"
      }
    ]
  }
]
```

See the [server.json schema](pkg/model/server.go) for full specification.

#### Other commands

```bash
# Run lint, unit tests and integration tests
make check
```

There are also a few more helpful commands for development. Run `make help` to learn more, or look in [Makefile](./Makefile).

<!--
For Claude and other AI tools: Always prefer make targets over custom commands where possible.
-->

## Architecture

### Project Structure

```
├── cmd/                     # Application entry points
│   └── registry/            # Registry server
├── data/                    # Seed data files
├── deploy/                  # Deployment configuration (Pulumi)
├── docs/                    # Documentation
├── internal/                # Private application code
│   ├── api/                 # HTTP handlers and routing
│   ├── config/              # Configuration management
│   ├── database/            # Data persistence (PostgreSQL)
│   ├── importer/            # Seed file import logic
│   ├── service/             # Business logic
│   ├── telemetry/           # Metrics and monitoring
│   └── validators/          # Input validation
├── pkg/                     # Public packages
│   ├── api/                 # API types and structures
│   │   └── v0/              # Version 0 API types
│   └── model/               # Data models for server.json
├── scripts/                 # Development and testing scripts
├── tests/                   # Integration tests
└── tools/                   # CLI tools and utilities
    └── validate-*.sh        # Schema validation tools
```

### API Endpoints

The registry provides the following read-only endpoints:

**Server Discovery:**
- `GET /v0/servers` - List all servers with pagination
- `GET /v0/servers/{name}/versions` - List versions of a specific server
- `GET /v0/servers/{name}/versions/{version}` - Get specific server version

**Utility:**
- `GET /v0/health` - Health check
- `GET /v0/ping` - Connectivity test
- `GET /v0/version` - Version information
- `POST /v0/validate` - Validate server.json without importing
- `GET /docs` - OpenAPI documentation

All endpoints are also available under `/v0.1` with the same functionality.

## Community Projects

Check out [community projects](docs/community-projects.md) to explore notable registry-related work created by the community.

## More documentation

See the [documentation](./docs) for more details if your question has not been answered here!
