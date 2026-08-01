// Pacote main: testes do ponto de entrada do serviço.
package main

import (
	"fmt"                         // Formatação da URL base do teste
	"log/slog"                    // Comparação dos níveis de log
	"net"                         // Reserva de porta efêmera para o teste
	"net/http"                    // Servidor fake da PokeAPI e verificação de health
	"net/http/httptest"           // Servidor HTTP de teste
	"os"                          // PID do processo para enviar o sinal
	"pokedex-api/internal/config" // Configuração usada em newLogger
	"syscall"                     // Constante do sinal SIGTERM
	"testing"                     // Framework de testes padrão do Go
	"time"                        // Controle de tempo nos testes
)

// netListenFreePort reserva uma porta efêmera e devolve seu número.
// A porta é liberada logo em seguida para o servidor usá-la.
func netListenFreePort() (int, error) {
	// Abre um listener em uma porta efêmera do SO.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// Propaga o erro se o listener não puder ser criado.
	if err != nil {
		return 0, err
	}
	// Obtém a porta reservada pelo SO.
	port := ln.Addr().(*net.TCPAddr).Port
	// Fecha o listener para liberar a porta ao servidor.
	_ = ln.Close()
	// Devolve a porta liberada sem erro.
	return port, nil
}

// TestRunInvalidConfig valida o retorno de erro com configuração inválida.
// A função run() deve propagar o erro do carregamento de configuração.
func TestRunInvalidConfig(t *testing.T) {
	// Define uma porta inválida para forçar o erro de configuração.
	t.Setenv("PORT", "nao-e-numero")
	// Executa run() esperando erro de configuração.
	if err := run(); err == nil {
		t.Error("esperava erro com configuração inválida")
	}
}

// TestNewLoggerText valida a criação do logger em formato texto.
func TestNewLoggerText(t *testing.T) {
	// Cria uma configuração com formato texto e nível DEBUG.
	cfg := config.Config{LogFormat: "text", LogLevel: slog.LevelDebug}
	// Cria o logger com a configuração de texto.
	logger := newLogger(cfg)
	// O logger não pode ser nulo.
	if logger == nil {
		t.Fatal("newLogger retornou nil para formato text")
	}
}

// TestNewLoggerJSON valida a criação do logger em formato JSON.
func TestNewLoggerJSON(t *testing.T) {
	// Cria uma configuração com formato JSON e nível WARN.
	cfg := config.Config{LogFormat: "json", LogLevel: slog.LevelWarn}
	// Cria o logger com a configuração de JSON.
	logger := newLogger(cfg)
	// O logger não pode ser nulo.
	if logger == nil {
		t.Fatal("newLogger retornou nil para formato json")
	}
}

// waitHealthy aguarda até o servidor responder 200 no /health.
// Usa um prazo fixo para não travar o teste indefinidamente.
func waitHealthy(t *testing.T, baseURL string) {
	// Define um prazo de 5 segundos para o servidor subir.
	deadline := time.Now().Add(5 * time.Second)
	// Laço de tentativas até o prazo expirar.
	for time.Now().Before(deadline) {
		// Faz a requisição de health contra o servidor.
		resp, err := http.Get(baseURL + "/health")
		// Se a requisição funcionou e respondeu 200, subiu.
		if err == nil {
			_ = resp.Body.Close() // Fecha o corpo da resposta
			if resp.StatusCode == http.StatusOK {
				return // Servidor pronto para receber requisições
			}
		}
		// Aguarda 50ms antes da próxima tentativa.
		time.Sleep(50 * time.Millisecond)
	}
	// Falha o teste se o servidor não subiu a tempo.
	t.Fatal("servidor não ficou pronto dentro do prazo")
}

// TestRunIntegrationWithSignal valida o ciclo de vida completo do main.
// Sobe o servidor real, verifica o health e encerra via SIGTERM.
func TestRunIntegrationWithSignal(t *testing.T) {
	// Cria o servidor fake da PokeAPI para o health retornar 200.
	poke := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Responde 200 com um corpo JSON mínimo.
		_, _ = w.Write([]byte(`{}`))
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer poke.Close()

	// Reserva uma porta efêmera para o servidor do main.
	port, err := netListenFreePort()
	// Falha o teste se não houver porta disponível.
	if err != nil {
		t.Fatalf("não foi possível reservar porta: %v", err)
	}
	// Monta a URL base para as verificações do teste.
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Configura o ambiente para o processo do serviço.
	t.Setenv("PORT", fmt.Sprintf("%d", port)) // Porta do servidor
	t.Setenv("POKEAPI_BASE_URL", poke.URL)    // PokeAPI fake
	t.Setenv("LOG_FORMAT", "text")            // Log em texto
	t.Setenv("CACHE_ENABLED", "true")         // Cache ligado
	t.Setenv("CACHE_TTL_SECONDS", "60")       // TTL de 60s
	t.Setenv("CACHE_MAX_ENTRIES", "10")       // Limite de 10

	// Canal para receber o retorno da função run.
	errCh := make(chan error, 1)
	// Inicia o run() em goroutine (bloqueia até o sinal chegar).
	go func() {
		errCh <- run() // Executa o serviço e envia o resultado
	}()

	// Aguarda o servidor responder no endpoint de health.
	waitHealthy(t, baseURL)
	// Envia SIGTERM para o processo (capturado pelo NotifyContext).
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)

	// Seleciona o retorno do run ou um timeout de segurança.
	select {
	case err := <-errCh:
		// O run deve encerrar sem erros após o sinal.
		if err != nil {
			t.Fatalf("run() retornou erro inesperado: %v", err)
		}
	case <-time.After(5 * time.Second):
		// Falha o teste se o encerramento demorar demais.
		t.Fatal("run() não encerrou dentro do prazo")
	}
}
