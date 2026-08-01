// Pacote server orquestra a composição do serviço: monta o cliente da
// PokeAPI, o cache, o roteador HTTP e administra o ciclo de vida do
// servidor (início e desligamento gracioso).
package server

import (
	"context"                         // Contexto de cancelamento do ciclo de vida
	"errors"                          // Comparação com http.ErrServerClosed
	"fmt"                             // Formatação de erros de inicialização
	"log/slog"                        // Logger compartilhado do serviço
	"net"                             // Listener injetável para testes
	"net/http"                        // Servidor HTTP
	"pokedex-api/internal/config"     // Configuração do serviço
	"pokedex-api/internal/docs"       // Documentação OpenAPI/Swagger embutida
	"pokedex-api/internal/httpserver" // Camada HTTP (router/handlers)
	"pokedex-api/internal/pokeapi"    // Cliente da PokeAPI
	"time"                            // Timeouts de leitura/escrita e do shutdown
)

// runOptions agrega as opções opcionais de execução do servidor.
// Atualmente permite injetar um listener (usado nos testes).
type runOptions struct {
	listener net.Listener // Listener pré-configurado (nil = criar padrão)
}

// Option é o tipo funcional para personalizar a execução do servidor.
type Option func(*runOptions)

// WithListener injeta um listener externo (por exemplo, criado em :0).
// Sem essa opção, o servidor cria seu próprio listener na porta da config.
func WithListener(l net.Listener) Option {
	// Retorna a função que grava o listener nas opções de execução.
	return func(o *runOptions) {
		o.listener = l // Define o listener informado
	}
}

// Run inicia o servidor HTTP e bloqueia até o contexto ser cancelado.
// Ao cancelar o contexto, faz um desligamento gracioso (drena conexões)
// e devolve o primeiro erro relevante encontrado no ciclo de vida.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger, opts ...Option) error {
	// Prepara as opções de execução aplicando as funções recebidas.
	ro := &runOptions{}
	// Itera sobre cada opção passada pelo chamador.
	for _, opt := range opts {
		opt(ro) // Aplica a opção nas opções de execução
	}

	// Cria o cache em memória apenas quando ele está habilitado.
	var cache *pokeapi.Cache // nil desabilita o cache no cliente
	if cfg.CacheEnabled {
		// Instancia o cache com TTL e limite vindos da configuração.
		cache = pokeapi.NewCache(cfg.CacheTTL, cfg.CacheMaxEntries)
	}

	// Cria o cliente da PokeAPI com cache, retentativas e logger.
	client := pokeapi.NewClient(cfg.PokeAPIBaseURL, cfg.RequestTimeout, cache, cfg.MaxRetries, logger)

	// Monta o handler HTTP final com todas as dependências injetadas.
	handler := httpserver.NewRouter(httpserver.Dependencies{
		Logger:      logger,                // Logger compartilhado
		Forwarder:   client,                // Proxy da PokeAPI
		Pinger:      client,                // Health check da PokeAPI
		Version:     cfg.Version,           // Versão do serviço
		CORSOrigin:  cfg.CORSAllowedOrigin, // Origem CORS configurada
		OpenAPISpec: docs.OpenAPISpec,      // Spec OpenAPI embutida
		SwaggerUI:   docs.SwaggerUI,        // Swagger UI embutido
	})

	// Monta o endereço de escuta a partir da porta configurada.
	addr := fmt.Sprintf(":%d", cfg.Port)
	// Cria o servidor HTTP com timeouts de proteção definidos.
	srv := &http.Server{
		Addr:              addr,             // Endereço de escuta
		Handler:           handler,          // Handler final da cadeia
		ReadHeaderTimeout: 5 * time.Second,  // Protege contra headers lentos
		IdleTimeout:       60 * time.Second, // Reaproveitamento de conexões
	}

	// Canal para receber o erro de execução do listener (tamanho 1).
	errCh := make(chan error, 1)
	// Inicia o servidor em uma goroutine separada (não bloqueante).
	go func() {
		// Se um listener externo foi injetado, serve nele.
		if ro.listener != nil {
			// Serve no listener pré-configurado (porta já definida).
			errCh <- srv.Serve(ro.listener)
		} else {
			// Sem listener injetado, cria o listener na porta config.
			errCh <- srv.ListenAndServe()
		}
	}()
	// Registra a inicialização bem-sucedida no log.
	logger.Info("servidor HTTP iniciado", "addr", addr, "version", cfg.Version)

	// Aguarda o cancelamento do contexto ou um erro de execução.
	select {
	case <-ctx.Done():
		// Contexto cancelado: inicia o desligamento gracioso.
	case err := <-errCh:
		// Erro real de execução (ex.: porta em uso) não pode ser ignorado.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err // Devolve o erro de inicialização/execução
		}
		// ErrServerClosed é esperado após o Shutdown: segue o fluxo.
	}

	// Cria um contexto com timeout para o desligamento gracioso.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Garante o cancelamento do contexto de shutdown ao final.
	defer cancel()
	// Inicia o desligamento gracioso (drena conexões ativas).
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Retorna erro caso o shutdown exceda o timeout.
		return fmt.Errorf("erro no desligamento gracioso: %w", err)
	}
	// Lê o erro final do canal de execução do listener.
	err := <-errCh
	// Se não for o erro esperado de servidor fechado, propaga.
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err // Erro inesperado durante o desligamento
	}
	// Ciclo de vida completo sem erros relevantes.
	return nil
}
