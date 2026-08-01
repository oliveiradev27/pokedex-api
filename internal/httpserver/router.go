// Pacote httpserver: router.go constrói o roteador HTTP completo do serviço,
// combinando o ServeMux do Go 1.22 com a cadeia de middlewares configurada.
package httpserver

import (
	"log/slog"                     // Logger compartilhado pelas dependências
	"net/http"                     // Interface HTTP padrão do Go
	"pokedex-api/internal/pokeapi" // Interfaces e tipos do contrato PokeAPI
)

// Dependencies agrega tudo o que os handlers precisam para funcionar.
// Centralizar as dependências facilita a construção do router e os testes.
type Dependencies struct {
	Logger      *slog.Logger      // Logger usado por handlers e middlewares
	Forwarder   pokeapi.Forwarder // Encaminhador de recursos da PokeAPI
	Pinger      pokeapi.Pinger    // Verificador de saúde da PokeAPI
	Version     string            // Versão do serviço (health e índice)
	CORSOrigin  string            // Origem permitida no CORS
	OpenAPISpec []byte            // Especificação OpenAPI embutida
	SwaggerUI   []byte            // Página do Swagger UI embutida
}

// NewRouter constrói o handler HTTP completo com todas as rotas e
// middlewares aplicados na ordem correta.
func NewRouter(deps Dependencies) http.Handler {
	// Cria o mux (roteador) nativo do net/http.
	mux := http.NewServeMux()

	// Instancia os handlers com as dependências injetadas.
	healthHandler := NewHealthHandler(deps.Pinger, deps.Version, deps.Logger) // Health check
	proxyHandler := NewProxyHandler(deps.Forwarder, deps.Logger)              // Proxy da PokeAPI
	docsHandler := NewDocsHandler(deps.OpenAPISpec, deps.SwaggerUI)           // Documentação
	indexHandler := NewIndexHandler(deps.Version)                             // Índice da API

	// Registra a rota do health check.
	mux.HandleFunc("GET /health", healthHandler.ServeHealth)
	// Registra a rota do proxy de recursos da PokeAPI.
	mux.HandleFunc("GET "+proxyResourcePattern, proxyHandler.ServeProxy)
	// Registra a rota que serve a especificação OpenAPI.
	mux.HandleFunc("GET /docs/openapi.json", docsHandler.ServeSpec)
	// Registra a rota que serve o Swagger UI.
	mux.HandleFunc("GET /docs", docsHandler.ServeUI)
	// Registra a rota raiz ($ exige o caminho exato "/").
	// Usar {$} evita que qualquer GET desconhecido caia no índice.
	mux.HandleFunc("GET /{$}", indexHandler.ServeIndex)

	// Monta a cadeia de middlewares (o mux fica no centro).
	var handler http.Handler = mux // Inicia a cadeia com o mux
	// Recuperação de panics: fica logo acima do mux para capturar tudo.
	handler = withRecovery(deps.Logger, handler)
	// Logging: registra status e duração de cada requisição.
	handler = withLogging(deps.Logger, handler)
	// Request ID: gera/injeta o ID antes do logging usá-lo.
	handler = withRequestID(handler)
	// CORS: aplicado por último, na camada mais externa.
	handler = withCORS(deps.CORSOrigin, handler)

	// Devolve o handler final já com toda a cadeia aplicada.
	return handler
}
