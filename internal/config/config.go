// Pacote config concentra toda a leitura de configuração do serviço.
// Todas as configurações são lidas de variáveis de ambiente, com valores
// padrão sensíveis para execução local, e são validadas no momento da carga.
package config

import (
	"errors"   // Fornece helper para criar erros simples
	"fmt"      // Formatação de erros com contexto
	"log/slog" // Níveis de log (DEBUG, INFO, WARN, ERROR)
	"net/url"  // Validação da URL base da PokeAPI
	"os"       // Leitura das variáveis de ambiente
	"strconv"  // Conversão de strings para números/booleanos
	"strings"  // Operações com strings (trim, comparação)
	"time"     // Tipos de duração para timeouts e TTL do cache
)

// Config agrega todas as opções de configuração do serviço em um único tipo.
// É um struct "value object": imutável depois de construído por Load().
type Config struct {
	Port              int           // Porta HTTP em que o servidor escuta
	PokeAPIBaseURL    string        // URL base da API externa (PokeAPI)
	RequestTimeout    time.Duration // Timeout máximo de cada requisição externa
	CacheEnabled      bool          // Habilita/desabilita o cache em memória
	CacheTTL          time.Duration // Tempo de vida de cada entrada no cache
	CacheMaxEntries   int           // Número máximo de entradas do cache
	MaxRetries        int           // Quantidade de tentativas extras no proxy
	LogLevel          slog.Level    // Nível mínimo de log emitido
	LogFormat         string        // Formato do log: "json" ou "text"
	CORSAllowedOrigin string        // Origem permitida no header CORS
	Version           string        // Versão do serviço exposta no /health
}

// Constantes com os valores padrão usados quando a env var não é definida.
// Centralizá-las aqui mantém o comportamento previsível e documentado.
const (
	defaultPort              = 8080                        // Porta padrão de execução
	defaultBaseURL           = "https://pokeapi.co/api/v2" // URL base oficial da PokeAPI
	defaultRequestTimeout    = 10 * time.Second            // Timeout padrão de 10s
	defaultCacheTTL          = 60 * time.Second            // Entradas válidas por 60s
	defaultCacheMaxEntries   = 512                         // Até 512 entradas em memória
	defaultMaxRetries        = 1                           // 1 tentativa extra após a inicial
	defaultLogLevel          = slog.LevelInfo              // Log informativo por padrão
	defaultLogFormat         = "json"                      // JSON é mais fácil de parsear
	defaultCORSAllowedOrigin = "*"                         // Permite qualquer origem
	defaultVersion           = "1.1.0"                     // Versão atual do serviço
)

// Load lê as variáveis de ambiente, aplica os padrões e valida os valores.
// Retorna (Config, nil) em caso de sucesso ou (Config{}, err) descrevendo o
// problema encontrado. Cada bloco abaixo trata uma variável específica.
func Load() (Config, error) {
	// Constroi a configuração inicial com todos os valores padrão definidos.
	cfg := Config{
		Port:              defaultPort,              // Porta padrão 8080
		PokeAPIBaseURL:    defaultBaseURL,           // PokeAPI oficial
		RequestTimeout:    defaultRequestTimeout,    // 10 segundos
		CacheEnabled:      true,                     // Cache ligado por padrão
		CacheTTL:          defaultCacheTTL,          // TTL de 60 segundos
		CacheMaxEntries:   defaultCacheMaxEntries,   // Limite de 512 entradas
		MaxRetries:        defaultMaxRetries,        // Uma retentativa
		LogLevel:          defaultLogLevel,          // Nível INFO
		LogFormat:         defaultLogFormat,         // Formato JSON
		CORSAllowedOrigin: defaultCORSAllowedOrigin, // Origem "*"
		Version:           defaultVersion,           // Versão 1.1.0
	}

	// ---- Variável PORT: porta de escuta do servidor HTTP ----
	// Apenas é processada quando a variável está presente no ambiente.
	if v := os.Getenv("PORT"); v != "" {
		// Converte a string do ambiente para inteiro.
		p, err := strconv.Atoi(v)
		// Se a conversão falhar, retorna erro com contexto claro.
		if err != nil {
			return Config{}, fmt.Errorf("PORT inválida %q: %w", v, err)
		}
		// Valida se a porta está dentro do intervalo válido de portas TCP.
		if p < 1 || p > 65535 {
			return Config{}, errors.New("PORT fora do intervalo válido (1-65535)")
		}
		// Aplica o valor validado à configuração.
		cfg.Port = p
	}

	// ---- Variável POKEAPI_BASE_URL: URL base da API externa ----
	if v := os.Getenv("POKEAPI_BASE_URL"); v != "" {
		// Faz o parse da URL para validar sintaxe e esquema.
		u, err := url.Parse(v)
		// Retorna erro se a URL não for parseável.
		if err != nil {
			return Config{}, fmt.Errorf("POKEAPI_BASE_URL inválida %q: %w", v, err)
		}
		// Garante que o esquema seja http ou https (segurança/consistência).
		if u.Scheme != "http" && u.Scheme != "https" {
			return Config{}, errors.New("POKEAPI_BASE_URL deve usar http ou https")
		}
		// Remove barras finais para evitar caminhos duplicados ("//").
		cfg.PokeAPIBaseURL = strings.TrimRight(v, "/")
	}

	// ---- Variável REQUEST_TIMEOUT_MS: timeout por requisição externa ----
	if v := os.Getenv("REQUEST_TIMEOUT_MS"); v != "" {
		// Converte o valor (em milissegundos) para inteiro.
		d, err := strconv.Atoi(v)
		// Propaga erro de conversão com contexto.
		if err != nil {
			return Config{}, fmt.Errorf("REQUEST_TIMEOUT_MS inválido %q: %w", v, err)
		}
		// Rejeita timeouts não positivos (evita requisições instantâneas).
		if d <= 0 {
			return Config{}, errors.New("REQUEST_TIMEOUT_MS deve ser maior que zero")
		}
		// Converte milissegundos para a duração nativa do Go.
		cfg.RequestTimeout = time.Duration(d) * time.Millisecond
	}

	// ---- Variável CACHE_ENABLED: liga ou desliga o cache em memória ----
	if v := os.Getenv("CACHE_ENABLED"); v != "" {
		// Converte "true"/"false" (e variantes) para booleano.
		b, err := strconv.ParseBool(v)
		// Propaga erro de conversão com contexto.
		if err != nil {
			return Config{}, fmt.Errorf("CACHE_ENABLED inválido %q: %w", v, err)
		}
		// Aplica o booleano convertido.
		cfg.CacheEnabled = b
	}

	// ---- Variável CACHE_TTL_SECONDS: validade de cada entrada no cache ----
	if v := os.Getenv("CACHE_TTL_SECONDS"); v != "" {
		// Converte o TTL (em segundos) para inteiro.
		d, err := strconv.Atoi(v)
		// Propaga erro de conversão com contexto.
		if err != nil {
			return Config{}, fmt.Errorf("CACHE_TTL_SECONDS inválido %q: %w", v, err)
		}
		// Rejeita TTLs não positivos.
		if d <= 0 {
			return Config{}, errors.New("CACHE_TTL_SECONDS deve ser maior que zero")
		}
		// Converte segundos para duração nativa do Go.
		cfg.CacheTTL = time.Duration(d) * time.Second
	}

	// ---- Variável CACHE_MAX_ENTRIES: limite de entradas do cache ----
	if v := os.Getenv("CACHE_MAX_ENTRIES"); v != "" {
		// Converte o limite para inteiro.
		n, err := strconv.Atoi(v)
		// Propaga erro de conversão com contexto.
		if err != nil {
			return Config{}, fmt.Errorf("CACHE_MAX_ENTRIES inválido %q: %w", v, err)
		}
		// Rejeita limites negativos (zero é aceito: cache sempre vazio).
		if n < 0 {
			return Config{}, errors.New("CACHE_MAX_ENTRIES não pode ser negativo")
		}
		// Aplica o limite validado.
		cfg.CacheMaxEntries = n
	}

	// ---- Variável MAX_RETRIES: tentativas extras de requisição externa ----
	if v := os.Getenv("MAX_RETRIES"); v != "" {
		// Converte o número de retentativas para inteiro.
		n, err := strconv.Atoi(v)
		// Propaga erro de conversão com contexto.
		if err != nil {
			return Config{}, fmt.Errorf("MAX_RETRIES inválido %q: %w", v, err)
		}
		// Rejeita valores negativos (zero desabilita as retentativas).
		if n < 0 {
			return Config{}, errors.New("MAX_RETRIES não pode ser negativo")
		}
		// Aplica o número de retentativas validado.
		cfg.MaxRetries = n
	}

	// ---- Variável LOG_LEVEL: nível mínimo de log (DEBUG/INFO/WARN/ERROR) ----
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		// LevelVar permite converter texto para slog.Level de forma nativa.
		var lvl slog.LevelVar
		// A conversão falha para valores desconhecidos como "verbose".
		if err := lvl.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("LOG_LEVEL inválido %q: %w", v, err)
		}
		// Extrai o nível resolvido do LevelVar.
		cfg.LogLevel = lvl.Level()
	}

	// ---- Variável LOG_FORMAT: formato de saída do log ----
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		// Apenas os formatos "json" e "text" são suportados pelo serviço.
		if v != "json" && v != "text" {
			return Config{}, errors.New("LOG_FORMAT deve ser \"json\" ou \"text\"")
		}
		// Aplica o formato validado.
		cfg.LogFormat = v
	}

	// ---- Variável CORS_ALLOWED_ORIGIN: origem permitida no CORS ----
	if v := os.Getenv("CORS_ALLOWED_ORIGIN"); v != "" {
		// Usa o valor exatamente como informado (ex.: "*" ou um domínio).
		cfg.CORSAllowedOrigin = v
	}

	// ---- Variável VERSION: versão do serviço exposta no /health ----
	if v := os.Getenv("VERSION"); v != "" {
		// Sobrescreve a versão padrão quando a variável é informada.
		cfg.Version = v
	}

	// Retorna a configuração completa e validada para o chamador.
	return cfg, nil
}
