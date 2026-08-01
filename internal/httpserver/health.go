// Pacote httpserver: health.go implementa o endpoint de verificação de
// saúde (/health). O check combina o estado do próprio serviço (liveness)
// com a disponibilidade da PokeAPI (readiness da dependência externa).
package httpserver

import (
	"context"                      // Timeout aplicado à verificação da dependência
	"log/slog"                     // Log estruturado de eventos de degradação
	"net/http"                     // Interface HTTP padrão do Go
	"pokedex-api/internal/pokeapi" // Interface Pinger do contrato PokeAPI
	"time"                         // Cálculo de uptime e timestamps
)

// pingTimeout define o tempo máximo para a verificação da dependência.
// Evita que o /health fique pendurado caso a PokeAPI não responda.
const pingTimeout = 5 * time.Second

// HealthHandler serve o endpoint de health check do serviço.
// As dependências são injetadas para permitir testes sem rede real.
type HealthHandler struct {
	pinger    pokeapi.Pinger // Verificador de disponibilidade da PokeAPI
	version   string         // Versão do serviço exposta no JSON
	startTime time.Time      // Instante de criação (para calcular uptime)
	logger    *slog.Logger   // Logger usado para registrar degradação
}

// NewHealthHandler constrói um HealthHandler com as dependências.
// startTime é capturado aqui para marcar o início do serviço.
func NewHealthHandler(pinger pokeapi.Pinger, version string, logger *slog.Logger) *HealthHandler {
	// Retorna a instância com o instante atual como início de uptime.
	return &HealthHandler{pinger: pinger, version: version, startTime: time.Now(), logger: logger}
}

// ServeHealth trata a rota GET /health.
// Responde 200 com status "ok" quando tudo está saudável e 503 com status
// "degraded" quando a PokeAPI está inacessível.
func (h *HealthHandler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	// Cria um contexto com timeout para a verificação da dependência.
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	// Garante o cancelamento do contexto ao final da função.
	defer cancel()

	// Valores padrão: serviço saudável e dependência de pé.
	depStatus := "ok"           // Estado da PokeAPI
	serviceStatus := "ok"       // Estado geral do serviço
	statusCode := http.StatusOK // Código HTTP 200

	// Verifica a disponibilidade da PokeAPI.
	if err := h.pinger.Ping(ctx); err != nil {
		// A dependência falhou: muda o estado geral para degradado.
		depStatus = "down"                         // Dependência indisponível
		serviceStatus = "degraded"                 // Serviço operando com problema
		statusCode = http.StatusServiceUnavailable // Código HTTP 503
		// Registra a degradação no log para diagnóstico posterior.
		h.logger.Warn("dependência PokeAPI indisponível", "err", err)
	}

	// Monta e escreve a resposta JSON do health check.
	writeJSON(w, statusCode, map[string]any{
		"status":         serviceStatus,                           // Estado geral do serviço
		"version":        h.version,                               // Versão do serviço
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),  // Tempo de atividade
		"dependencies":   map[string]string{"pokeapi": depStatus}, // Estado das dependências
		"timestamp":      time.Now().UTC().Format(time.RFC3339),   // Momento da resposta
	})
}
