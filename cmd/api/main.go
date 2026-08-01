// cmd/api é o ponto de entrada do serviço pokedex-api.
// O main.go é propositalmente enxuto: a lógica vive em internal/server,
// o que mantém este pacote simples e testável.
package main

import (
	"context"                     // Contexto raiz do ciclo de vida
	"log/slog"                    // Logger padrão de nível raiz
	"os"                          // Sinalizadores e saída padrão
	"os/signal"                   // Captura de sinais de interrupção
	"pokedex-api/internal/config" // Leitura da configuração do serviço
	"pokedex-api/internal/server" // Inicialização do servidor HTTP
	"syscall"                     // Constantes de sinais (SIGTERM)
)

// main é a porta de entrada: executa o serviço e sai com código 1 em erro.
// Nenhuma lógica de negócio vive aqui (apenas orquestração).
func main() {
	// Executa o serviço e verifica se houve erro fatal.
	if err := run(); err != nil {
		// Registra o erro fatal no logger padrão.
		slog.Error("erro fatal", "err", err)
		// Sai do processo com código de erro 1.
		os.Exit(1)
	}
}

// run carrega a configuração, monta o logger e inicia o servidor.
// O contexto é cancelado ao receber SIGINT (Ctrl+C) ou SIGTERM.
func run() error {
	// Carrega a configuração a partir das variáveis de ambiente.
	cfg, err := config.Load()
	// Se a configuração for inválida, interrompe a execução.
	if err != nil {
		return err
	}

	// Cria o logger a partir da configuração carregada.
	logger := newLogger(cfg)

	// Cria um contexto cancelado ao receber sinais de interrupção.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Garante a liberação dos handlers de sinal ao final.
	defer stop()

	// Inicia o servidor e bloqueia até o desligamento gracioso.
	return server.Run(ctx, cfg, logger)
}

// newLogger constrói o logger do serviço conforme o formato configurado.
// "json" produz logs estruturados; qualquer outro valor produz texto plano.
func newLogger(cfg config.Config) *slog.Logger {
	// LevelVar permite alterar o nível de log em tempo de execução.
	var level slog.LevelVar
	// Define o nível mínimo de log vindo da configuração.
	level.Set(cfg.LogLevel)
	// Cria as opções do handler de log com o nível configurado.
	opts := &slog.HandlerOptions{Level: &level}

	// Define o handler de log de acordo com o formato configurado.
	var handler slog.Handler
	// Se o formato for texto, usa o handler de texto legível.
	if cfg.LogFormat == "text" {
		// Cria o handler de log em texto para stdout.
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Caso contrário, usa o handler de log estruturado em JSON.
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	// Cria o logger final com o handler escolhido.
	return slog.New(handler)
}
