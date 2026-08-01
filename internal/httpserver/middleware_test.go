// Pacote httpserver: testes dos middlewares da cadeia HTTP.
package httpserver

import (
	"bytes"             // Buffer para capturar a saída do logger
	"encoding/json"     // Decodificação das respostas JSON
	"io"                // Descarte de saída do logger silencioso
	"log/slog"          // Logger usado nos middlewares
	"net/http"          // Interface HTTP padrão do Go
	"net/http/httptest" // Servidor/recorder de teste
	"strings"           // Verificação de conteúdo nas respostas
	"testing"           // Framework de testes padrão do Go
)

// newDiscardLogger retorna um logger que descarta toda a saída.
func newDiscardLogger() *slog.Logger {
	// Cria um logger de texto apontando para o descarte de bytes.
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestWithRequestIDWithoutHeader valida a geração de um novo ID.
// Quando o cliente não envia o header, um ID deve ser criado e retornado.
func TestWithRequestIDWithoutHeader(t *testing.T) {
	// Captura o ID gerado e definido no header da resposta.
	var gotID string
	// Cria um handler que lê o ID do contexto e guarda na variável.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = requestIDFrom(r)     // Extrai o ID do contexto
		w.WriteHeader(http.StatusOK) // Responde 200
	})
	// Cria a requisição sem o header X-Request-ID.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o middleware com o handler de teste.
	withRequestID(next).ServeHTTP(rec, req)
	// O ID deve ter sido gerado (não pode ser vazio).
	if gotID == "" {
		t.Error("request ID não foi gerado")
	}
	// O mesmo ID deve aparecer no header da resposta.
	if rec.Header().Get(requestIDHeader) != gotID {
		t.Errorf("header %s = %q; esperado %q", requestIDHeader, rec.Header().Get(requestIDHeader), gotID)
	}
}

// TestWithRequestIDWithHeader valida a reutilização do ID do cliente.
func TestWithRequestIDWithHeader(t *testing.T) {
	// Captura o ID lido do contexto pelo handler interno.
	var gotID string
	// Cria um handler que registra o ID do contexto.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = requestIDFrom(r) // Extrai o ID do contexto
	})
	// Cria a requisição com o header X-Request-ID preenchido.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Define o ID informado pelo cliente.
	req.Header.Set(requestIDHeader, "meu-id-customizado")
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o middleware com o handler de teste.
	withRequestID(next).ServeHTTP(rec, req)
	// O ID do cliente deve ser preservado no contexto.
	if gotID != "meu-id-customizado" {
		t.Errorf("ID = %q; esperado meu-id-customizado", gotID)
	}
}

// TestWithLoggingCapturesRequest valida o log estruturado das requisições.
// Verifica que método, caminho e status aparecem na saída do log.
func TestWithLoggingCapturesRequest(t *testing.T) {
	// Buffer que captura a saída do logger em teste.
	var buf bytes.Buffer
	// Cria um logger JSON escrevendo no buffer capturado.
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	// Cria um handler que responde 201 para testar a captura de status.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // Status 201
	})
	// Cria a requisição POST para o caminho /api/v1/pokemon/1.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pokemon/1", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o middleware de logging sobre o handler de teste.
	withLogging(logger, next).ServeHTTP(rec, req)
	// O log deve conter o método HTTP da requisição.
	if !strings.Contains(buf.String(), `"method":"POST"`) {
		t.Errorf("log não contém o método: %s", buf.String())
	}
	// O log deve conter o caminho da requisição.
	if !strings.Contains(buf.String(), `"path":"/api/v1/pokemon/1"`) {
		t.Errorf("log não contém o caminho: %s", buf.String())
	}
	// O log deve conter o status final capturado.
	if !strings.Contains(buf.String(), `"status":201`) {
		t.Errorf("log não contém o status: %s", buf.String())
	}
}

// TestWithRecoveryCatchesPanic valida que panics viram resposta 500.
func TestWithRecoveryCatchesPanic(t *testing.T) {
	// Cria um handler que lança um panic deliberadamente.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("explosão intencional") // Panic simulado
	})
	// Cria a requisição GET para qualquer caminho.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o middleware de recovery sobre o handler com panic.
	withRecovery(newDiscardLogger(), next).ServeHTTP(rec, req)
	// A resposta deve ser 500 Internal Server Error.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Code = %d; esperado 500", rec.Code)
	}
	// O corpo deve ser JSON de erro válido.
	var body map[string]string
	// Decodifica o corpo da resposta capturada.
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// A mensagem de erro deve estar presente no corpo.
	if body["error"] == "" {
		t.Error("mensagem de erro ausente no corpo")
	}
}

// TestWithCORSHeaders valida os headers CORS e o preflight OPTIONS.
func TestWithCORSHeaders(t *testing.T) {
	// Cria um handler simples que responde 200.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // Resposta normal
	})
	// Cria um handler já envolvido pelo middleware de CORS.
	h := withCORS("*", next)

	// Cenário 1: requisição normal GET deve ter os headers CORS.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o middleware com a requisição GET.
	h.ServeHTTP(rec, req)
	// Verifica o header de origem permitida.
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("header CORS de origem ausente ou incorreto")
	}
	// Verifica o header de métodos permitidos.
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("header CORS de métodos ausente")
	}
	// Verifica o header de headers permitidos.
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("header CORS de headers ausente")
	}

	// Cenário 2: preflight OPTIONS deve responder 204 e parar a cadeia.
	reqOpt := httptest.NewRequest(http.MethodOptions, "/health", nil)
	// Cria o recorder para capturar o preflight.
	recOpt := httptest.NewRecorder()
	// Executa o middleware com a requisição OPTIONS.
	h.ServeHTTP(recOpt, reqOpt)
	// O preflight deve retornar 204 No Content.
	if recOpt.Code != http.StatusNoContent {
		t.Errorf("Code = %d; esperado 204 para OPTIONS", recOpt.Code)
	}
}
