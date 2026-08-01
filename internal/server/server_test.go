// Pacote server: testes do ciclo de vida do servidor HTTP.
// Validam a inicialização, o atendimento de requisições e o shutdown.
package server

import (
	"context"                     // Contextos de cancelamento do ciclo de vida
	"encoding/json"               // Decodificação da resposta de health
	"fmt"                         // Formatação de URLs nos testes
	"io"                          // Descarte de saída do logger
	"log/slog"                    // Logger silencioso dos testes
	"net"                         // Criação de listeners de teste
	"net/http"                    // Requisições de verificação ao servidor
	"pokedex-api/internal/config" // Configuração do serviço
	"testing"                     // Framework de testes padrão do Go
	"time"                        // Controle de tempo nos testes
)

// newTestConfig monta uma configuração válida com a porta informada.
func newTestConfig(port int) config.Config {
	// Retorna uma configuração mínima para os testes do servidor.
	return config.Config{
		Port:              port,                 // Porta de escuta
		PokeAPIBaseURL:    "http://127.0.0.1:1", // Base URL não rastreável
		RequestTimeout:    time.Second,          // Timeout curto de teste
		CacheEnabled:      true,                 // Cache habilitado
		CacheTTL:          time.Minute,          // TTL de 1 minuto
		CacheMaxEntries:   10,                   // Limite de 10 entradas
		MaxRetries:        0,                    // Sem retentativas
		LogLevel:          slog.LevelInfo,       // Nível INFO
		LogFormat:         "json",               // Formato JSON
		CORSAllowedOrigin: "*",                  // Origem aberta
		Version:           "test",               // Versão de teste
	}
}

// freePort reserva uma porta efêmera e devolve seu número.
// A porta é liberada logo em seguida para o servidor usá-la.
func freePort(t *testing.T) int {
	// Abre um listener em uma porta efêmera do SO.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// Falha o teste se nenhuma porta puder ser reservada.
	if err != nil {
		t.Fatalf("não foi possível reservar porta: %v", err)
	}
	// Obtém a porta reservada pelo SO.
	port := ln.Addr().(*net.TCPAddr).Port
	// Fecha o listener para liberar a porta ao servidor.
	_ = ln.Close()
	// Devolve a porta liberada.
	return port
}

// newDiscardLogger retorna um logger silencioso para os testes.
func newDiscardLogger() *slog.Logger {
	// Cria um logger de texto apontando para o descarte de bytes.
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitHealthy aguarda até o servidor HTTP aceitar requisições.
// Qualquer resposta HTTP (mesmo 503) indica que o servidor está de pé;
// o estado do health em si é coberto nos testes do handler.
func waitHealthy(t *testing.T, baseURL string) {
	// Define um prazo de 5 segundos para o servidor subir.
	deadline := time.Now().Add(5 * time.Second)
	// Laço de tentativas até o prazo expirar.
	for time.Now().Before(deadline) {
		// Faz a requisição de health contra o servidor.
		resp, err := http.Get(baseURL + "/health")
		// Se a requisição obteve resposta, o servidor está no ar.
		if err == nil {
			_ = resp.Body.Close() // Fecha o corpo da resposta
			return                // Servidor pronto para requisições
		}
		// Aguarda 50ms antes da próxima tentativa.
		time.Sleep(50 * time.Millisecond)
	}
	// Falha o teste se o servidor não subiu a tempo.
	t.Fatal("servidor não ficou pronto dentro do prazo")
}

// TestRunLifecycle valida o ciclo de vida completo do servidor.
// Inicia com listener injetado, faz requisições e encerra graciosamente.
func TestRunLifecycle(t *testing.T) {
	// Cria um listener em porta efêmera para o servidor.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// Falha o teste se o listener não puder ser criado.
	if err != nil {
		t.Fatalf("não foi possível criar listener: %v", err)
	}
	// Obtém a porta real do listener criado.
	port := ln.Addr().(*net.TCPAddr).Port
	// Monta a URL base para as verificações do teste.
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	// Cria a configuração com a porta do listener.
	cfg := newTestConfig(port)
	// Cria um contexto cancelável para controlar o shutdown.
	ctx, cancel := context.WithCancel(context.Background())

	// Canal para receber o retorno da função Run.
	errCh := make(chan error, 1)
	// Inicia o servidor em goroutine com o listener injetado.
	go func() {
		// Executa o servidor e envia o resultado para o canal.
		errCh <- Run(ctx, cfg, newDiscardLogger(), WithListener(ln))
	}()

	// Aguarda o servidor responder no endpoint de health.
	waitHealthy(t, baseURL)
	// Faz uma requisição ao proxy para validar o atendimento.
	resp, err := http.Get(baseURL + "/api/v1/pokemon/1")
	// Verifica se a requisição ao proxy funcionou.
	if err != nil {
		t.Fatalf("requisição ao proxy falhou: %v", err)
	}
	// Fecha o corpo da resposta do proxy.
	_ = resp.Body.Close()
	// O status 502 é esperado (PokeAPI fake não está disponível).
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status do proxy = %d; esperado 502", resp.StatusCode)
	}

	// Cancela o contexto para iniciar o desligamento gracioso.
	cancel()
	// Seleciona o retorno do Run ou um timeout de segurança.
	select {
	case err := <-errCh:
		// O servidor deve encerrar sem erros.
		if err != nil {
			t.Fatalf("Run() retornou erro inesperado: %v", err)
		}
	case <-time.After(5 * time.Second):
		// Falha o teste se o desligamento demorar demais.
		t.Fatal("servidor não encerrou dentro do prazo")
	}
}

// TestRunPortConflict valida o erro quando a porta já está em uso.
// O Run deve retornar o erro de bind do listener padrão.
func TestRunPortConflict(t *testing.T) {
	// Reserva uma porta efêmera em todas as interfaces e a mantém ocupada.
	// Usar ":0" (todas as interfaces) garante conflito real com o bind de
	// ":%d" feito por ListenAndServe (SO_REUSEADDR do macOS permitiria o
	// segundo bind caso a reserva fosse apenas em 127.0.0.1).
	ln, err := net.Listen("tcp", ":0")
	// Falha o teste se o listener não puder ser criado.
	if err != nil {
		t.Fatalf("não foi possível criar listener: %v", err)
	}
	// Garante o fechamento do listener ao final do teste.
	defer func() { _ = ln.Close() }()
	// Obtém a porta do listener que será ocupada.
	port := ln.Addr().(*net.TCPAddr).Port
	// Cria a configuração apontando para a porta ocupada.
	cfg := newTestConfig(port)
	// Executa o Run sem listener injetado (usa ListenAndServe).
	err = Run(context.Background(), cfg, newDiscardLogger())
	// Deve retornar um erro de porta já em uso.
	if err == nil {
		t.Error("esperava erro de porta em uso")
	}
}

// TestRunHealthResponse valida o conteúdo real do /health no servidor.
// Verifica o JSON estruturado retornado pelo endpoint ao vivo.
func TestRunHealthResponse(t *testing.T) {
	// Cria um listener em porta efêmera para o servidor.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// Falha o teste se o listener não puder ser criado.
	if err != nil {
		t.Fatalf("não foi possível criar listener: %v", err)
	}
	// Obtém a porta real do listener criado.
	port := ln.Addr().(*net.TCPAddr).Port
	// Monta a URL base para as verificações do teste.
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	// Cria a configuração com a porta do listener.
	cfg := newTestConfig(port)
	// Cria um contexto cancelável para controlar o shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	// Garante o cancelamento ao final do teste.
	defer cancel()
	// Inicia o servidor em goroutine com o listener injetado.
	errCh := make(chan error, 1)
	// Executa o servidor e envia o resultado para o canal.
	go func() { errCh <- Run(ctx, cfg, newDiscardLogger(), WithListener(ln)) }()

	// Aguarda o servidor responder no endpoint de health.
	waitHealthy(t, baseURL)
	// Faz a requisição ao endpoint de health.
	resp, err := http.Get(baseURL + "/health")
	// Falha o teste se a requisição não funcionar.
	if err != nil {
		t.Fatalf("requisição ao health falhou: %v", err)
	}
	// Garante o fechamento do corpo da resposta.
	defer func() { _ = resp.Body.Close() }()
	// Decodifica o corpo do health check.
	var body map[string]any
	// Decodifica o JSON recebido do servidor.
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// O campo status deve existir na resposta.
	if _, ok := body["status"]; !ok {
		t.Error("campo status ausente na resposta de health")
	}
	// O campo version deve existir na resposta.
	if _, ok := body["version"]; !ok {
		t.Error("campo version ausente na resposta de health")
	}
	// O campo uptime_seconds deve existir na resposta.
	if _, ok := body["uptime_seconds"]; !ok {
		t.Error("campo uptime_seconds ausente na resposta de health")
	}
}
