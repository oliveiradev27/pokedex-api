// Pacote config_test contém os testes unitários do pacote config.
// Eles validam valores padrão, sobreposição por ambiente e casos de erro.
package config

import (
	"log/slog" // Comparação dos níveis de log nos testes
	"testing"  // Framework de testes padrão do Go
	"time"     // Comparação de durações nos testes
)

// TestDefaults garante que, sem variáveis de ambiente, todos os padrões
// sejam aplicados corretamente ao carregar a configuração.
func TestDefaults(t *testing.T) {
	// Carrega a configuração sem nenhuma variável de ambiente definida.
	cfg, err := Load()
	// Falha o teste se o Load retornar algum erro inesperado.
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}
	// Cria um mapa com os valores padrão esperados para comparação.
	want := map[string]any{
		"Port":              defaultPort,              // Porta padrão
		"PokeAPIBaseURL":    defaultBaseURL,           // URL padrão
		"RequestTimeout":    defaultRequestTimeout,    // Timeout padrão
		"CacheEnabled":      true,                     // Cache habilitado
		"CacheTTL":          defaultCacheTTL,          // TTL padrão
		"CacheMaxEntries":   defaultCacheMaxEntries,   // Limite padrão
		"MaxRetries":        defaultMaxRetries,        // Retentativas padrão
		"LogLevel":          defaultLogLevel,          // Nível padrão
		"LogFormat":         defaultLogFormat,         // Formato padrão
		"CORSAllowedOrigin": defaultCORSAllowedOrigin, // Origem padrão
		"Version":           defaultVersion,           // Versão padrão
	}
	// Monta um mapa com os valores reais obtidos da configuração.
	got := map[string]any{
		"Port":              cfg.Port,              // Porta real
		"PokeAPIBaseURL":    cfg.PokeAPIBaseURL,    // URL real
		"RequestTimeout":    cfg.RequestTimeout,    // Timeout real
		"CacheEnabled":      cfg.CacheEnabled,      // Cache real
		"CacheTTL":          cfg.CacheTTL,          // TTL real
		"CacheMaxEntries":   cfg.CacheMaxEntries,   // Limite real
		"MaxRetries":        cfg.MaxRetries,        // Retentativas reais
		"LogLevel":          cfg.LogLevel,          // Nível real
		"LogFormat":         cfg.LogFormat,         // Formato real
		"CORSAllowedOrigin": cfg.CORSAllowedOrigin, // Origem real
		"Version":           cfg.Version,           // Versão real
	}
	// Itera sobre todos os campos para comparar real contra esperado.
	for key, wantValue := range want {
		// Verifica se o valor real difere do esperado para a chave atual.
		if got[key] != wantValue {
			// Reporta a divergência com ambos os valores no erro.
			t.Errorf("campo %s = %v; esperado %v", key, got[key], wantValue)
		}
	}
}

// TestLoadWithEnvOverrides valida que todas as variáveis de ambiente são
// respeitadas quando informadas com valores válidos.
func TestLoadWithEnvOverrides(t *testing.T) {
	// Define todas as variáveis com valores válidos e distintos.
	t.Setenv("PORT", "9090")                                   // Porta alternativa
	t.Setenv("POKEAPI_BASE_URL", "http://localhost:9999/api/") // URL com barra final
	t.Setenv("REQUEST_TIMEOUT_MS", "2500")                     // Timeout de 2.5s
	t.Setenv("CACHE_ENABLED", "false")                         // Cache desligado
	t.Setenv("CACHE_TTL_SECONDS", "30")                        // TTL de 30s
	t.Setenv("CACHE_MAX_ENTRIES", "10")                        // Limite de 10 entradas
	t.Setenv("MAX_RETRIES", "3")                               // Três retentativas
	t.Setenv("LOG_LEVEL", "DEBUG")                             // Nível DEBUG
	t.Setenv("LOG_FORMAT", "text")                             // Formato texto
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://exemplo.com")     // Origem específica
	t.Setenv("VERSION", "9.9.9")                               // Versão customizada

	// Carrega a configuração com todas as variáveis aplicadas.
	cfg, err := Load()
	// Falha o teste se algum valor inválido escapar da validação.
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}
	// Valores esperados após aplicar as variáveis de ambiente.
	if cfg.Port != 9090 { // Porta deve ser 9090
		t.Errorf("Port = %d; esperado 9090", cfg.Port)
	}
	// A URL deve ter a barra final removida pela normalização.
	if cfg.PokeAPIBaseURL != "http://localhost:9999/api" {
		t.Errorf("PokeAPIBaseURL = %q; esperado http://localhost:9999/api", cfg.PokeAPIBaseURL)
	}
	// Timeout deve ser exatamente 2500 milissegundos.
	if cfg.RequestTimeout != 2500*time.Millisecond {
		t.Errorf("RequestTimeout = %v; esperado 2500ms", cfg.RequestTimeout)
	}
	// Cache deve estar desabilitado pela variável.
	if cfg.CacheEnabled {
		t.Error("CacheEnabled = true; esperado false")
	}
	// TTL deve ser de 30 segundos.
	if cfg.CacheTTL != 30*time.Second {
		t.Errorf("CacheTTL = %v; esperado 30s", cfg.CacheTTL)
	}
	// Limite de entradas deve ser 10.
	if cfg.CacheMaxEntries != 10 {
		t.Errorf("CacheMaxEntries = %d; esperado 10", cfg.CacheMaxEntries)
	}
	// Retentativas devem ser 3.
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d; esperado 3", cfg.MaxRetries)
	}
	// Nível de log deve ser DEBUG.
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v; esperado DEBUG", cfg.LogLevel)
	}
	// Formato deve ser texto.
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q; esperado text", cfg.LogFormat)
	}
	// Origem CORS deve ser o domínio informado.
	if cfg.CORSAllowedOrigin != "https://exemplo.com" {
		t.Errorf("CORSAllowedOrigin = %q; esperado https://exemplo.com", cfg.CORSAllowedOrigin)
	}
	// Versão deve ser a customizada.
	if cfg.Version != "9.9.9" {
		t.Errorf("Version = %q; esperado 9.9.9", cfg.Version)
	}
}

// TestLoadInvalidPort cobre os dois erros possíveis para a variável PORT:
// valor não numérico e valor fora do intervalo válido.
func TestLoadInvalidPort(t *testing.T) {
	// Define a PORT como um valor não numérico.
	t.Setenv("PORT", "abc")
	// Carrega esperando que o erro de conversão seja retornado.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para PORT não numérica")
	}

	// Reseta a PORT para um número fora do intervalo válido.
	t.Setenv("PORT", "70000")
	// Carrega esperando o erro de intervalo.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para PORT fora do intervalo")
	}
}

// TestLoadInvalidBaseURL cobre os erros de URL inválida: esquema proibido
// e string completamente mal formada.
func TestLoadInvalidBaseURL(t *testing.T) {
	// Usa um esquema (ftp) que não é permitido pelo serviço.
	t.Setenv("POKEAPI_BASE_URL", "ftp://pokeapi.co")
	// Carrega esperando o erro de esquema.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para esquema ftp")
	}

	// Usa uma string claramente inválida como URL.
	t.Setenv("POKEAPI_BASE_URL", "://::://")
	// Carrega esperando o erro de parse.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para URL mal formada")
	}
}

// TestLoadInvalidNumbers cobre os erros de conversão e de valor para as
// variáveis numéricas restantes do serviço.
func TestLoadInvalidNumbers(t *testing.T) {
	// Caso 1: REQUEST_TIMEOUT_MS com valor não numérico.
	t.Setenv("REQUEST_TIMEOUT_MS", "rápido")
	// Carrega esperando erro de conversão.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para REQUEST_TIMEOUT_MS não numérico")
	}
	// Caso 2: REQUEST_TIMEOUT_MS zerado é rejeitado.
	t.Setenv("REQUEST_TIMEOUT_MS", "0")
	// Carrega esperando erro de valor não positivo.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para REQUEST_TIMEOUT_MS zerado")
	}

	// Caso 3: CACHE_TTL_SECONDS não numérico.
	t.Setenv("REQUEST_TIMEOUT_MS", "")
	t.Setenv("CACHE_TTL_SECONDS", "logo")
	// Carrega esperando erro de conversão.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CACHE_TTL_SECONDS não numérico")
	}
	// Caso 4: CACHE_TTL_SECONDS negativo é rejeitado.
	t.Setenv("CACHE_TTL_SECONDS", "-5")
	// Carrega esperando erro de valor não positivo.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CACHE_TTL_SECONDS negativo")
	}

	// Caso 5: CACHE_MAX_ENTRIES não numérico.
	t.Setenv("CACHE_TTL_SECONDS", "")
	t.Setenv("CACHE_MAX_ENTRIES", "muitas")
	// Carrega esperando erro de conversão.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CACHE_MAX_ENTRIES não numérico")
	}
	// Caso 6: CACHE_MAX_ENTRIES negativo é rejeitado.
	t.Setenv("CACHE_MAX_ENTRIES", "-1")
	// Carrega esperando erro de valor negativo.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CACHE_MAX_ENTRIES negativo")
	}

	// Caso 7: MAX_RETRIES não numérico.
	t.Setenv("CACHE_MAX_ENTRIES", "")
	t.Setenv("MAX_RETRIES", "varias")
	// Carrega esperando erro de conversão.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para MAX_RETRIES não numérico")
	}
	// Caso 8: MAX_RETRIES negativo é rejeitado.
	t.Setenv("MAX_RETRIES", "-1")
	// Carrega esperando erro de valor negativo.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para MAX_RETRIES negativo")
	}
}

// TestLoadInvalidMisc cobre os erros de validação das variáveis restantes:
// booleano inválido, nível de log desconhecido e formato de log proibido.
func TestLoadInvalidMisc(t *testing.T) {
	// Caso 1: CACHE_ENABLED com valor não booleano.
	t.Setenv("CACHE_ENABLED", "talvez")
	// Carrega esperando erro de conversão booleana.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CACHE_ENABLED inválido")
	}
	// Caso 2: LOG_LEVEL com valor desconhecido.
	t.Setenv("CACHE_ENABLED", "")
	t.Setenv("LOG_LEVEL", "verboso")
	// Carrega esperando erro de nível desconhecido.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para LOG_LEVEL inválido")
	}
	// Caso 3: LOG_FORMAT com valor não suportado.
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "xml")
	// Carrega esperando erro de formato não suportado.
	if _, err := Load(); err == nil {
		t.Error("esperava erro para LOG_FORMAT inválido")
	}
}
