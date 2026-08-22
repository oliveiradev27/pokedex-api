// Pacote pokeapi: testes do cliente HTTP que conversa com a PokeAPI.
// Usam um servidor HTTP fake (httptest) para simular a API externa.
package pokeapi

import (
	"context"           // Contextos de cancelamento nos testes
	"encoding/json"     // Parsing do modelo Pokemon nos testes
	"io"                // Destino de descarte do log
	"log/slog"          // Logger descartável (io.Discard)
	"net/http"          // Criação de handlers do servidor fake
	"net/http/httptest" // Servidor HTTP de teste
	"sync/atomic"       // Contadores atômicos de requisições
	"testing"           // Framework de testes padrão do Go
	"time"              // Controle de timeout nos testes
)

// newTestLogger retorna um logger que descarta toda a saída.
// Mantém os testes silenciosos e sem dependência do terminal.
func newTestLogger() *slog.Logger {
	// Cria um logger de texto apontando para o descarte de bytes.
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestForwardSuccess valida o fluxo feliz de encaminhamento de um recurso.
func TestForwardSuccess(t *testing.T) {
	// Define o corpo JSON que o servidor fake deve retornar.
	body := `{"name":"pikachu","id":25}`
	// Cria o servidor fake que responde com o corpo definido.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define o content type da resposta fake.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Escreve o corpo da resposta fake.
		_, _ = w.Write([]byte(body))
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake (sem cache).
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Encaminha a requisição do recurso pokemon/pikachu.
	resp, err := c.Forward(context.Background(), "pokemon/pikachu", "")
	// Falha o teste se o encaminhamento retornar erro.
	if err != nil {
		t.Fatalf("Forward() retornou erro: %v", err)
	}
	// Verifica o status HTTP da resposta fake.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d; esperado 200", resp.StatusCode)
	}
	// Verifica o content type repassado da resposta fake.
	if resp.ContentType != "application/json; charset=utf-8" {
		t.Errorf("ContentType = %q", resp.ContentType)
	}
	// Verifica o corpo repassado da resposta fake.
	var parsed Pokemon
	if err := json.Unmarshal(resp.Body, &parsed); err != nil || parsed.ID != 25 || parsed.Name != "pikachu" {
		t.Errorf("Body não foi convertido para Pokemon: %q", resp.Body)
	}
}

func TestParseAndSerializePokemon(t *testing.T) {
	body := []byte(`{"id":25,"name":"pikachu","height":4,"weight":60,"types":[{"slot":1,"type":{"name":"electric","url":"https://pokeapi.co/api/v2/type/13/"}}]}`)
	got, err := ParseAndSerialize("pokemon/pikachu", body)
	if err != nil {
		t.Fatalf("ParseAndSerialize() retornou erro: %v", err)
	}
	var pokemon Pokemon
	if err := json.Unmarshal(got, &pokemon); err != nil {
		t.Fatalf("JSON serializado inválido: %v", err)
	}
	if pokemon.ID != 25 || pokemon.Name != "pikachu" || len(pokemon.Types) != 1 {
		t.Fatalf("pokemon parseado incorretamente: %+v", pokemon)
	}
}

func TestParseAndSerializeRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseAndSerialize("pokemon/1", []byte(`{invalid`)); err == nil {
		t.Fatal("esperava erro para JSON inválido")
	}
}

func TestParseAndSerializeBerryAndItem(t *testing.T) {
	berry, err := ParseAndSerialize("berry/cheri", []byte(`{"id":1,"name":"cheri","growth_time":3}`))
	if err != nil {
		t.Fatalf("berry: %v", err)
	}
	var gotBerry Berry
	if err := json.Unmarshal(berry, &gotBerry); err != nil || gotBerry.Name != "cheri" || gotBerry.ID != 1 {
		t.Fatalf("berry inválida: %s", berry)
	}
	item, err := ParseAndSerialize("item/potion", []byte(`{"id":17,"name":"potion","cost":300}`))
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	var gotItem Item
	if err := json.Unmarshal(item, &gotItem); err != nil || gotItem.Name != "potion" || gotItem.Cost != 300 {
		t.Fatalf("item inválido: %s", item)
	}
}

// TestForwardForwardsQuery valida que a query string é repassada à PokeAPI.
func TestForwardForwardsQuery(t *testing.T) {
	// Guarda a query recebida pelo servidor fake para verificação.
	var gotQuery atomic.Value
	// Cria o servidor fake que registra a query recebida.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Armazena a query string recebida na requisição.
		gotQuery.Store(r.URL.RawQuery)
		// Escreve uma resposta 200 mínima.
		w.WriteHeader(http.StatusOK)
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake.
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Encaminha uma listagem com limit e offset na query.
	_, err := c.Forward(context.Background(), "pokemon", "limit=5&offset=10")
	// Falha o teste se o encaminhamento retornar erro.
	if err != nil {
		t.Fatalf("Forward() retornou erro: %v", err)
	}
	// Lê a query capturada pelo servidor fake.
	q, _ := gotQuery.Load().(string)
	// Verifica se a query foi repassada integralmente.
	if q != "limit=5&offset=10" {
		t.Errorf("query repassada = %q; esperado limit=5&offset=10", q)
	}
}

// TestForwardNotFound valida o repasse de status 404 da PokeAPI.
// O proxy não deve transformar o 404 da origem em erro.
func TestForwardNotFound(t *testing.T) {
	// Cria o servidor fake que sempre responde 404.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define o status 404 para o recurso inexistente.
		w.WriteHeader(http.StatusNotFound)
		// Escreve o corpo de erro da PokeAPI.
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake.
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Encaminha um recurso que não existe na API fake.
	resp, err := c.Forward(context.Background(), "pokemon/999999", "")
	// Falha o teste se o encaminhamento retornar erro.
	if err != nil {
		t.Fatalf("Forward() retornou erro: %v", err)
	}
	// O status 404 deve ser repassado como resposta final.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d; esperado 404", resp.StatusCode)
	}
}

// TestForwardNetworkError valida o erro quando a PokeAPI está inacessível.
// Usa um servidor já encerrado para simular conexão recusada.
func TestForwardNetworkError(t *testing.T) {
	// Cria um servidor fake que será encerrado imediatamente.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // Handler nunca será chamado
	}))
	// Encerra o servidor para que as conexões sejam recusadas.
	srv.Close()
	// Cria o cliente apontando para o servidor encerrado.
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Encaminha um recurso esperando falha de conexão.
	_, err := c.Forward(context.Background(), "pokemon/1", "")
	// O erro de rede deve ser retornado nesse cenário.
	if err == nil {
		t.Fatal("esperava erro de rede para servidor encerrado")
	}
}

// TestForwardContextCancel valida que o cancelamento do contexto aborta
// a requisição de forma limpa.
func TestForwardContextCancel(t *testing.T) {
	// Cria o servidor fake que bloqueia a resposta (nunca responde).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Aguarda até o contexto ser cancelado ou 2s (máximo).
		select {
		case <-r.Context().Done(): // Contexto cancelado pelo cliente
		case <-time.After(2 * time.Second): // Timeout de segurança
		}
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake.
	c := NewClient(srv.URL, 5*time.Second, nil, 0, newTestLogger())
	// Cria um contexto que será cancelado imediatamente.
	ctx, cancel := context.WithCancel(context.Background())
	// Cancela o contexto antes mesmo de iniciar a requisição.
	cancel()
	// Encaminha esperando o erro de cancelamento de contexto.
	_, err := c.Forward(ctx, "pokemon/1", "")
	// O erro deve ser a propagação do cancelamento.
	if err == nil {
		t.Fatal("esperava erro de contexto cancelado")
	}
}

// TestForwardRetryThenSuccess valida a política de retentativas.
// A primeira tentativa falha com 500 e a segunda deve obter sucesso.
func TestForwardRetryThenSuccess(t *testing.T) {
	// Contador atômico do número de requisições recebidas.
	var attempts int32
	// Cria o servidor fake que falha na primeira e acerta na segunda.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Incrementa o contador de tentativas.
		if atomic.AddInt32(&attempts, 1) == 1 {
			// Na primeira chamada responde 500 (erro temporário).
			w.WriteHeader(http.StatusInternalServerError)
			return // Encerra o handler da primeira tentativa
		}
		// Nas chamadas seguintes responde 200 com sucesso.
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente com 1 retentativa extra.
	c := NewClient(srv.URL, time.Second, nil, 1, newTestLogger())
	// Encaminha o recurso esperando sucesso após a retentativa.
	resp, err := c.Forward(context.Background(), "pokemon/1", "")
	// Falha o teste se todas as tentativas falharem.
	if err != nil {
		t.Fatalf("Forward() retornou erro: %v", err)
	}
	// Verifica se o status final é o esperado (200).
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d; esperado 200", resp.StatusCode)
	}
	// Verifica se foram realizadas exatamente 2 tentativas.
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("attempts = %d; esperado 2", attempts)
	}
}

// TestForwardAllRetriesFail valida o repasse do 5xx após todas as tentativas.
// Nenhum erro é gerado; o status 500 final é repassado como resposta.
func TestForwardAllRetriesFail(t *testing.T) {
	// Cria o servidor fake que sempre responde 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Responde 500 em todas as tentativas.
		w.WriteHeader(http.StatusInternalServerError)
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente com 2 retentativas (3 tentativas no total).
	c := NewClient(srv.URL, time.Second, nil, 2, newTestLogger())
	// Encaminha o recurso que sempre falha.
	resp, err := c.Forward(context.Background(), "pokemon/1", "")
	// Nenhum erro deve ser retornado (o 500 é repassado).
	if err != nil {
		t.Fatalf("Forward() retornou erro inesperado: %v", err)
	}
	// O status 500 final deve ser repassado como resposta.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d; esperado 500", resp.StatusCode)
	}
}

// TestForwardCacheHit valida que a segunda chamada usa o cache.
// O servidor fake conta as chamadas: deve receber apenas 1.
func TestForwardCacheHit(t *testing.T) {
	// Contador atômico de requisições recebidas pelo servidor.
	var requests int32
	// Cria o servidor fake que conta e responde 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Incrementa o contador de requisições.
		atomic.AddInt32(&requests, 1)
		// Escreve a resposta 200 com um corpo fixo.
		_, _ = w.Write([]byte(`{"cached":"no"}`))
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cache com TTL generoso para o teste.
	cache := NewCache(time.Minute, 10)
	// Cria o cliente com o cache habilitado.
	c := NewClient(srv.URL, time.Second, cache, 0, newTestLogger())
	// Primeira chamada: deve ir até o servidor fake.
	if _, err := c.Forward(context.Background(), "pokemon/1", ""); err != nil {
		t.Fatalf("primeira Forward() falhou: %v", err)
	}
	// Segunda chamada: deve ser servida pelo cache.
	if _, err := c.Forward(context.Background(), "pokemon/1", ""); err != nil {
		t.Fatalf("segunda Forward() falhou: %v", err)
	}
	// O servidor fake deve ter recebido apenas uma requisição.
	if n := atomic.LoadInt32(&requests); n != 1 {
		t.Errorf("requests = %d; esperado 1 (cache deveria atender)", n)
	}
}

// TestPingSuccess valida que o ping retorna nil quando a API está de pé.
func TestPingSuccess(t *testing.T) {
	// Cria o servidor fake que responde 200 no ping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // API saudável
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake.
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Executa o ping esperando sucesso.
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() retornou erro: %v", err)
	}
}

// TestPingError valida que o ping retorna erro quando a API responde 5xx.
func TestPingError(t *testing.T) {
	// Cria o servidor fake que responde 503 no ping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // API indisponível
	}))
	// Garante o encerramento do servidor fake ao final do teste.
	defer srv.Close()
	// Cria o cliente apontando para o servidor fake.
	c := NewClient(srv.URL, time.Second, nil, 0, newTestLogger())
	// Executa o ping esperando erro de status inesperado.
	if err := c.Ping(context.Background()); err == nil {
		t.Error("esperava erro para ping com status 503")
	}
}

// TestPingNetworkError valida o erro de rede no ping.
// Usa um endereço inválido para simular a API fora do ar.
func TestPingNetworkError(t *testing.T) {
	// Cria um cliente apontando para um endereço não rastreável.
	c := NewClient("http://127.0.0.1:1", 100*time.Millisecond, nil, 0, newTestLogger())
	// Executa o ping esperando falha de conexão.
	if err := c.Ping(context.Background()); err == nil {
		t.Error("esperava erro de rede para porta 1")
	}
}

// TestRetryDelayBounds valida os limites do backoff exponencial com jitter.
// O delay deve ficar entre o valor base e o teto máximo mais o jitter.
func TestRetryDelayBounds(t *testing.T) {
	// Itera sobre vários índices de tentativa do backoff.
	for attempt := 1; attempt <= 6; attempt++ {
		// Calcula o delay para a tentativa atual.
		delay := retryDelay(attempt)
		// O delay nunca pode ser menor que o valor base.
		if delay < retryBaseDelay {
			t.Errorf("attempt %d: delay %v menor que o base %v", attempt, delay, retryBaseDelay)
		}
		// O delay máximo possível é o teto mais o jitter máximo.
		maxExpected := retryMaxDelay + retryBaseDelay
		// O delay não pode ultrapassar o teto com jitter.
		if delay > maxExpected {
			t.Errorf("attempt %d: delay %v excede o teto %v", attempt, delay, maxExpected)
		}
	}
}

// TestForwardInvalidURL valida o erro quando a URL de recurso é inválida.
// O erro de junção de URL é propagado para o chamador.
func TestForwardInvalidURL(t *testing.T) {
	// Cria o cliente com uma base URL mal formada.
	c := NewClient("://invalida", time.Second, nil, 0, newTestLogger())
	// Encaminha um recurso esperando erro de URL.
	if _, err := c.Forward(context.Background(), "pokemon/1", ""); err == nil {
		t.Error("esperava erro para URL base inválida")
	}
}
