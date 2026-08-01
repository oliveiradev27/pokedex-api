# Pokedex API Proxy

[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://go.dev/dl/)

A simple yet robust API proxy written in Go, designed to consume the [PokeAPI](https://pokeapi.co/docs/v2) and serve its data verbatim. It includes essential features like a health check endpoint, OpenAPI/Swagger documentation, in-memory caching, request retries, and graceful shutdown.

## 🚀 Features

*   **PokeAPI Proxy:** Transparently forwards requests to the official PokeAPI, serving JSON responses directly. Supports resource lookups (e.g., `/api/v1/pokemon/pikachu`) and paginated lists (e.g., `/api/v1/pokemon?limit=5`).
*   **Health Check:** A `/health` endpoint provides service status and verifies the availability of the PokeAPI dependency.
*   **OpenAPI/Swagger Documentation:** An OpenAPI 3.0 specification is available at `/docs/openapi.json`, and an interactive Swagger UI can be accessed at `/docs`.
*   **In-Memory Cache:** Caches 2xx responses from the PokeAPI with configurable TTL and limits to reduce load on the upstream API. Supports thread-safe access and lazy eviction of expired entries.
*   **Intelligent Retries:** Implements exponential backoff with jitter for handling temporary failures (5xx status codes or network errors) when communicating with the PokeAPI.
*   **Graceful Shutdown:** Ensures that the server shuts down cleanly, draining active connections and respecting context cancellation (SIGINT/SIGTERM).
*   **Environment-Driven Configuration:** All settings (port, timeouts, cache behavior, logging) are configurable via environment variables.
*   **Go Standard Library Only:** Built entirely using Go's standard library, ensuring zero external dependencies for a predictable and self-contained deployment.
*   **High Test Coverage:** Achieves over 95% code coverage with comprehensive unit and integration tests.

## 🛠️ Stack

*   **Language:** Go 1.26.4
*   **Core Libraries:** `net/http` (including Go 1.22+ path patterns), `log/slog`, `//go:embed`, `context`, `sync`, `time`, `net`.

## 📍 Endpoints

| Método | Rota                  | Descrição                                         | Respostas                                                        |
| :----- | :-------------------- | :------------------------------------------------ | :--------------------------------------------------------------- |
| `GET`  | `/health`             | Health check do serviço e da PokeAPI              | `200` ok / `503` degraded                                        |
| `GET`  | `/api/v1/{resource}/{id}` | Proxy pass-through de recurso da PokeAPI        | `200` (recurso), `400` (path inválido), `404` (repassado), `502` (falha de rede) |
| `GET`  | `/docs`               | Interface Swagger UI interativa                   | `200` HTML                                                       |
| `GET`  | `/docs/openapi.json`  | Especificação OpenAPI 3.0 em JSON                 | `200` JSON                                                       |
| `GET`  | `/`                   | Índice da API (nome, versão, endpoints)           | `200` JSON                                                       |

> `{resource}` e `{id}` podem ser nomes ou IDs numéricos da PokeAPI (ex: `pokemon/pikachu`, `berry/1`, `item/potion`).

## ⚙️ Configuration

All settings are configurable via environment variables:

| Variable                | Default                   | Description                                       |
| :---------------------- | :------------------------ | :------------------------------------------------ |
| `PORT`                  | `8080`                    | HTTP server port.                                 |
| `POKEAPI_BASE_URL`      | `https://pokeapi.co/api/v2` | Base URL for the external PokeAPI.                |
| `REQUEST_TIMEOUT_MS`    | `10000`                   | Timeout for external requests (milliseconds).     |
| `CACHE_ENABLED`         | `true`                    | Enable/disable in-memory cache.                   |
| `CACHE_TTL_SECONDS`     | `60`                      | Cache entry Time-To-Live (seconds).               |
| `CACHE_MAX_ENTRIES`     | `512`                     | Maximum number of entries in the cache.           |
| `MAX_RETRIES`           | `1`                       | Number of extra retries for 5xx/network errors.   |
| `LOG_LEVEL`             | `INFO`                    | Minimum log level (`DEBUG`, `INFO`, `WARN`, `ERROR`). |
| `LOG_FORMAT`            | `json`                    | Log output format (`json` or `text`).             |
| `CORS_ALLOWED_ORIGIN`   | `*`                       | Allowed origin for CORS headers. **Use specific origins in production.** |
| `VERSION`               | `1.1.0`                   | Service version exposed in `/health` and index.   |

## 🚀 Setup & Execution

### Prerequisites
*   **Go 1.22+** (tested with Go 1.26.4). Download from [go.dev/dl/](https://go.dev/dl/).

### Quick Start
```bash
# 1. Clone the repository and navigate to the project directory
git clone <repository-url>
cd pokedex-api

# 2. (Optional) Compile the binary
make build

# 3. Run the server (defaults to port 8080)
make run

# Or run with custom environment variables:
# PORT=9090 CACHE_TTL_SECONDS=30 LOG_FORMAT=text go run ./cmd/api
