// Pacote httpserver: testes do endpoint de health check.
package httpserver

import (
	"context"           // Tipo usado na assinatura da interface Pinger
	"encoding/json"     // Decodificação da resposta JSON
	"errors"            // Criação de erros fake para o pinger stub
	"net/http"          // Interface HTTP padrão do Go
	"net/http/httptest" // Servidor/recorder de teste
	"testing"           // Framework de testes padrão do Go
)

// stubPinger é um fake da interface Pinger para os testes de health.
// Configura o erro que a verificação de dependência deve devolver.
type stubPinger struct {
	err error // Erro fake devolvido pelo Ping
}

// Ping implementa a interface Pinger devolvendo o erro configurado.
func (s *stubPinger) Ping(context.Context) error {
	// Devolve o erro fake configurado (nil = dependência saudável).
	return s.err
}

// TestHealthOK valida a resposta 200 quando a PokeAPI está de pé.
func TestHealthOK(t *testing.T) {
	// Cria o pinger stub sem erro (dependência saudável).
	pinger := &stubPinger{err: nil}
	// Instancia o handler de health com o stub e versão fixa.
	h := NewHealthHandler(pinger, "1.2.3", newDiscardLogger())
	// Cria a requisição GET para o health check.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o handler de health.
	h.ServeHealth(rec, req)
	// O status deve ser 200 OK.
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// Decodifica o corpo da resposta para inspeção.
	var body map[string]any
	// Decodifica o corpo capturado no recorder.
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// O status geral deve ser "ok".
	if body["status"] != "ok" {
		t.Errorf("status = %v; esperado ok", body["status"])
	}
	// A versão deve ser a configurada.
	if body["version"] != "1.2.3" {
		t.Errorf("version = %v; esperado 1.2.3", body["version"])
	}
	// A dependência pokeapi deve estar "ok".
	deps := body["dependencies"].(map[string]any)
	// Verifica o estado da dependência dentro do objeto aninhado.
	if deps["pokeapi"] != "ok" {
		t.Errorf("pokeapi = %v; esperado ok", deps["pokeapi"])
	}
}

// TestHealthDegraded valida a resposta 503 quando a PokeAPI está fora.
func TestHealthDegraded(t *testing.T) {
	// Cria o pinger stub com erro (dependência indisponível).
	pinger := &stubPinger{err: errors.New("conexão recusada")}
	// Instancia o handler de health com o stub e versão fixa.
	h := NewHealthHandler(pinger, "1.2.3", newDiscardLogger())
	// Cria a requisição GET para o health check.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o handler de health.
	h.ServeHealth(rec, req)
	// O status deve ser 503 Service Unavailable.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Code = %d; esperado 503", rec.Code)
	}
	// Decodifica o corpo da resposta para inspeção.
	var body map[string]any
	// Decodifica o corpo capturado no recorder.
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// O status geral deve ser "degraded".
	if body["status"] != "degraded" {
		t.Errorf("status = %v; esperado degraded", body["status"])
	}
	// A dependência pokeapi deve estar "down".
	deps := body["dependencies"].(map[string]any)
	// Verifica o estado da dependência dentro do objeto aninhado.
	if deps["pokeapi"] != "down" {
		t.Errorf("pokeapi = %v; esperado down", deps["pokeapi"])
	}
}
