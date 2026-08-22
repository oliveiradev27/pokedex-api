// Pacote pokeapi implementa o cliente HTTP que conversa com a PokeAPI.
// Ele expõe a operação de encaminhamento (proxy) de recursos e a operação
// de ping (verificação de saúde) usada pelo endpoint /health.
package pokeapi

import (
	"context"      // Propagação de cancelamento/timeout para as requisições
	"fmt"          // Formatação de mensagens de erro
	"io"           // Leitura do corpo da resposta
	"log/slog"     // Log estruturado das operações do cliente
	"math/rand/v2" // Jitter aleatório para o backoff das retentativas
	"net/http"     // Cliente HTTP e tipos de requisição/resposta
	"net/url"      // Junção e validação de caminhos de URL
	"strings"      // Operações de trim e prefixo em strings
	"time"         // Timeout do cliente HTTP e contagem de retentativas
)

// maxResponseBytes limita o tamanho do corpo lido da PokeAPI.
// Evita que uma resposta anômala consuma toda a memória do processo.
const maxResponseBytes int64 = 10 << 20 // 10 MiB é suficiente para qualquer recurso

// retryBaseDelay é o delay base do backoff exponencial entre tentativas.
// Cada tentativa k usa base * 2^(k-1), sempre com um teto máximo.
const retryBaseDelay = 100 * time.Millisecond

// retryMaxDelay é o teto máximo do backoff (o delay não cresce além dele).
// Evita esperas excessivas em cenários de falhas prolongadas.
const retryMaxDelay = 2 * time.Second

// retryDelay calcula o atraso entre tentativas k e k+1 da retentativa.
// Usa backoff exponencial (base * 2^(k-1)) com teto e jitter aleatório.
// O jitter dispersa os novos ataques da rede, evitando "ondas" sincronizadas.
func retryDelay(attempt int) time.Duration {
	// Calcula o valor exponencial dobrando o base a cada tentativa.
	delay := retryBaseDelay * time.Duration(1<<(attempt-1))
	// Aplica o teto máximo para o delay não crescer indefinidamente.
	if delay > retryMaxDelay {
		delay = retryMaxDelay
	}
	// Adiciona um jitter aleatório entre 0 e o delay base.
	// rand/v2 é seguro para uso concorrente entre goroutines.
	return delay + time.Duration(rand.Int64N(int64(retryBaseDelay)))
}

// Response representa a resposta da PokeAPI pronta para ser repassada.
// Contém status, content type e o corpo já lido (para cache/passagem).
type Response struct {
	StatusCode  int    // Código de status HTTP vindo da PokeAPI
	ContentType string // Header Content-Type vindo da PokeAPI
	Body        []byte // Corpo da resposta já serializado em bytes
}

// Forwarder é a interface de encaminhamento usada pelos handlers HTTP.
// O pacote httpserver consome esta interface; o Client a implementa.
type Forwarder interface {
	// Forward encaminha um recurso da PokeAPI retornando a resposta.
	// resourcePath é o caminho relativo (ex.: "pokemon/pikachu") e
	// rawQuery é a query string original (ex.: "limit=5&offset=10").
	Forward(ctx context.Context, resourcePath, rawQuery string) (*Response, error)
}

// Pinger é a interface de verificação de saúde usada pelo /health.
// Permite injetar uma implementação falsa nos testes do handler.
type Pinger interface {
	// Ping verifica se a PokeAPI está acessível e saudável.
	// Retorna nil quando a dependência responde corretamente.
	Ping(ctx context.Context) error
}

// Client é a implementação concreta do Forwarder/Pinger sobre a PokeAPI.
// Agrega um http.Client, um cache opcional e a política de retentativas.
type Client struct {
	baseURL    string       // URL base da PokeAPI (sem barra final)
	httpClient *http.Client // Cliente HTTP com timeout configurado
	cache      *Cache       // Cache opcional (nil desabilita o cache)
	maxRetries int          // Número de tentativas extras no proxy
	logger     *slog.Logger // Logger usado para registrar operações
}

// NewClient constrói um novo Client com as dependências injetadas.
// baseURL é normalizado (barra final removida) antes do uso.
func NewClient(baseURL string, timeout time.Duration, cache *Cache, maxRetries int, logger *slog.Logger) *Client {
	// Cria a instância com todos os campos inicializados.
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"), // Remove barra final
		httpClient: &http.Client{Timeout: timeout},  // Aplica o timeout global
		cache:      cache,                           // Guarda o cache (pode ser nil)
		maxRetries: maxRetries,                      // Guarda o número de tentativas
		logger:     logger,                          // Guarda o logger injetado
	}
}

// Forward implementa a interface Forwarder: busca um recurso na PokeAPI.
// A estratégia é: 1) consultar o cache; 2) requisitar com retentativas;
// 3) armazenar no cache quando a resposta for 2xx e o cache estiver ativo.
func (c *Client) Forward(ctx context.Context, resourcePath, rawQuery string) (*Response, error) {
	// Limpa a barra inicial do caminho para montar a URL corretamente.
	cleanPath := strings.TrimLeft(resourcePath, "/")
	// Monta a chave do cache combinando caminho e query string.
	cacheKey := cleanPath
	// Se houver query, inclui na chave para diferenciar listagens.
	if rawQuery != "" {
		cacheKey = cleanPath + "?" + rawQuery
	}
	// Consulta o cache primeiro, se ele estiver habilitado.
	if c.cache != nil {
		// Tenta obter uma entrada válida do cache.
		if hit, ok := c.cache.Get(cacheKey); ok {
			// Registra a operação como servida pelo cache.
			c.logger.Debug("cache hit", "path", cleanPath)
			// Devolve a resposta do cache sem chamar a PokeAPI.
			return hit, nil
		}
		// Registra o miss para observabilidade do tráfego real à PokeAPI.
		c.logger.Debug("cache miss", "path", cleanPath)
	}

	// Monta a URL absoluta do recurso na PokeAPI.
	u, err := url.JoinPath(c.baseURL, cleanPath)
	// Se a junção falhar, não há como prosseguir com a requisição.
	if err != nil {
		return nil, fmt.Errorf("URL inválida para %q: %w", cleanPath, err)
	}
	// Adiciona a query string original à URL montada.
	if rawQuery != "" {
		u += "?" + rawQuery
	}

	// Executa o laço de requisição com as retentativas configuradas.
	var lastErr error // Armazena o último erro para a mensagem final
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Entre tentativas, aplica um backoff exponencial com jitter.
		if attempt > 0 {
			// Espera o delay calculado, mas respeita o cancelamento.
			select {
			case <-ctx.Done(): // Se o contexto for cancelado...
				return nil, ctx.Err() // ...devolve o erro de cancelamento.
			case <-time.After(retryDelay(attempt)): // Backoff exponencial
			}
		}

		// Executa a requisição HTTP de fato para a tentativa atual.
		resp, err := c.doGet(ctx, u)
		// Em caso de erro de rede, guarda e tenta novamente (se houver).
		if err != nil {
			lastErr = err                                                     // Registra o erro da tentativa
			c.logger.Warn("tentativa falhou", "attempt", attempt, "err", err) // Loga
			continue                                                          // Avança para a próxima tentativa
		}
		// Se for erro 5xx e ainda há tentativas, considera falha temporária.
		if resp.StatusCode >= 500 && attempt < c.maxRetries {
			// Guarda o status para diagnóstico na mensagem de erro.
			lastErr = fmt.Errorf("status %d da PokeAPI", resp.StatusCode)
			// Loga a falha temporária com o status recebido.
			c.logger.Warn("status 5xx, retentando", "attempt", attempt, "status", resp.StatusCode)
			// Pula o cache e parte para a próxima tentativa.
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(resp.Body) > 0 {
			parsedBody, parseErr := ParseAndSerialize(cleanPath, resp.Body)
			if parseErr != nil {
				return nil, parseErr
			}
			resp.Body = parsedBody
		}

		// Resposta válida obtida: armazena no cache quando é 2xx.
		if c.cache != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Registra o recurso no cache com o TTL configurado.
			c.cache.Set(cacheKey, resp)
		}
		// Devolve a resposta final (mesmo que seja um 4xx/5xx definitivo).
		return resp, nil
	}

	// Esgotou as tentativas com erros de rede: devolve erro explicativo.
	return nil, fmt.Errorf("falha após %d tentativa(s): %w", c.maxRetries+1, lastErr)
}

// doGet executa uma única requisição GET e lê o corpo da resposta.
// A leitura é limitada por maxResponseBytes para proteger a memória.
func (c *Client) doGet(ctx context.Context, url string) (*Response, error) {
	// Cria a requisição GET com o contexto (timeout/cancelamento) herdado.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	// Propaga qualquer erro de construção da requisição.
	if err != nil {
		return nil, err
	}
	// Envia a requisição usando o cliente HTTP configurado.
	resp, err := c.httpClient.Do(req)
	// Propaga erros de rede (conexão recusada, timeout, DNS, etc.).
	if err != nil {
		return nil, err
	}
	// Garante que o corpo sempre seja fechado, mesmo em erro de leitura.
	defer resp.Body.Close()
	// Lê o corpo respeitando o limite máximo de bytes.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	// Propaga erros de leitura do corpo da resposta.
	if err != nil {
		return nil, err
	}
	// Obtém o content type vindo da PokeAPI (pode ser vazio).
	contentType := resp.Header.Get("Content-Type")
	// Se o content type vier vazio, assume JSON (padrão da API).
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	// Monta e devolve a resposta estruturada para o chamador.
	return &Response{
		StatusCode:  resp.StatusCode, // Preserva o status original
		ContentType: contentType,     // Preserva o content type original
		Body:        body,            // Preserva o corpo lido
	}, nil
}

// Ping implementa a interface Pinger: verifica a disponibilidade da API.
// Uma resposta 2xx indica que a PokeAPI está acessível e saudável.
func (c *Client) Ping(ctx context.Context) error {
	// Cria a requisição GET contra a raiz da PokeAPI.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	// Propaga erros de construção da requisição de ping.
	if err != nil {
		return err
	}
	// Envia a requisição de ping.
	resp, err := c.httpClient.Do(req)
	// Propaga erros de rede como indisponibilidade.
	if err != nil {
		return err
	}
	// Garante o fechamento do corpo da resposta de ping.
	defer resp.Body.Close()
	// Descarta o corpo lendo até o fim (poucos bytes na raiz da API).
	_, _ = io.Copy(io.Discard, resp.Body)
	// Se o status for 2xx, a API está saudável.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // Sem erro: ping bem-sucedido.
	}
	// Caso contrário, devolve erro descrevendo o status inesperado.
	return fmt.Errorf("PokeAPI respondeu com status %d", resp.StatusCode)
}
