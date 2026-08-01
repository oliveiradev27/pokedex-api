// Pacote pokeapi: testes do cache TTL em memória.
package pokeapi

import (
	"sync"    // Sincronização do teste concorrente
	"testing" // Framework de testes padrão do Go
	"time"    // Controle de expiração nos testes
)

// newTestResponse monta uma resposta fake simples para popular o cache.
func newTestResponse(status int, body string) *Response {
	// Retorna uma resposta com status, content type e corpo fornecidos.
	return &Response{StatusCode: status, ContentType: "application/json", Body: []byte(body)}
}

// TestCacheSetGet valida o fluxo básico de escrita e leitura no cache.
func TestCacheSetGet(t *testing.T) {
	// Cria um cache com TTL generoso e limite padrão.
	c := NewCache(time.Hour, 10)
	// Cria a resposta fake para armazenar.
	resp := newTestResponse(200, `{"id":1}`)
	// Armazena a resposta sob a chave "pokemon/1".
	c.Set("pokemon/1", resp)
	// Recupera a resposta armazenada sob a mesma chave.
	got, ok := c.Get("pokemon/1")
	// Falha o teste se a entrada não for encontrada.
	if !ok {
		t.Fatal("esperava encontrar a chave no cache")
	}
	// Verifica se o conteúdo do corpo foi preservado.
	if string(got.Body) != `{"id":1}` {
		t.Errorf("Body = %q; esperado {\"id\":1}", got.Body)
	}
	// Verifica se a contagem de entradas é exatamente 1.
	if c.Len() != 1 {
		t.Errorf("Len = %d; esperado 1", c.Len())
	}
	// Chaves inexistentes devem retornar ausentes.
	if _, ok := c.Get("pokemon/2"); ok {
		t.Error("chave inexistente não deveria ser encontrada")
	}
}

// TestCacheExpired valida que entradas vencidas são tratadas como miss.
// Usa um relógio fake para expirar a entrada sem esperar o TTL real.
func TestCacheExpired(t *testing.T) {
	// Cria um cache com TTL de 1 minuto e limite padrão.
	c := NewCache(time.Minute, 10)
	// Define o relógio fake começando em um instante fixo.
	fakeNow := time.Unix(1000, 0)
	// Injeta o relógio fake no cache (campo acessível no mesmo pacote).
	c.now = func() time.Time { return fakeNow }
	// Armazena uma resposta com o relógio fake em 1000s.
	c.Set("k", newTestResponse(200, "a"))

	// Dentro do TTL, a entrada deve ser retornada.
	if _, ok := c.Get("k"); !ok {
		t.Error("entrada dentro do TTL deveria ser retornada")
	}

	// Avança o relógio fake além do TTL (60s depois).
	fakeNow = fakeNow.Add(time.Minute + time.Second)
	// A entrada deve ser considerada ausente após a expiração.
	if _, ok := c.Get("k"); ok {
		t.Error("entrada expirada não deveria ser retornada")
	}
	// A entrada expirada deve ter sido removida (limpeza lazy).
	if c.Len() != 0 {
		t.Errorf("Len = %d; esperado 0 após remoção lazy", c.Len())
	}
}

// TestCacheEviction valida o comportamento de expiração por capacidade.
// Quando o limite é atingido, a entrada mais antiga/expirada é removida.
func TestCacheEviction(t *testing.T) {
	// Cria um cache com capacidade máxima de 2 entradas.
	c := NewCache(time.Hour, 2)
	// Preenche até o limite de 2 entradas.
	c.Set("a", newTestResponse(200, "a"))
	c.Set("b", newTestResponse(200, "b"))
	// A terceira inserção deve expulsar uma entrada existente.
	c.Set("c", newTestResponse(200, "c"))
	// O número de entradas não pode ultrapassar o limite configurado.
	if c.Len() > 2 {
		t.Errorf("Len = %d; esperado no máximo 2", c.Len())
	}
	// Após a expulsão, uma das chaves antigas deve ter sumido.
	if c.Len() != 2 {
		t.Errorf("Len = %d; esperado 2 após eviction", c.Len())
	}
}

// TestCacheEvictionPrefersExpired valida que entradas expiradas são as
// primeiras a serem removidas quando o cache atinge o limite.
func TestCacheEvictionPrefersExpired(t *testing.T) {
	// Cria um cache com capacidade de 1 entrada.
	c := NewCache(time.Minute, 1)
	// Define o relógio fake para controlar a expiração.
	fakeNow := time.Unix(0, 0)
	// Injeta o relógio fake no cache.
	c.now = func() time.Time { return fakeNow }
	// Armazena a primeira entrada ainda válida.
	c.Set("a", newTestResponse(200, "a"))
	// Expira a primeira entrada avançando o relógio além do TTL.
	fakeNow = fakeNow.Add(2 * time.Minute)
	// Armazena a segunda entrada (força a remoção da expirada).
	c.Set("b", newTestResponse(200, "b"))
	// A entrada expirada deve ter sido a removida.
	if _, ok := c.Get("a"); ok {
		t.Error("entrada expirada deveria ter sido removida")
	}
	// A entrada nova deve estar presente e válida.
	if _, ok := c.Get("b"); !ok {
		t.Error("entrada nova deveria estar no cache")
	}
}

// TestCacheConcurrent garante que o cache é seguro para uso concorrente.
// Múltiplas goroutines escrevem e leem simultaneamente sem corridas.
func TestCacheConcurrent(t *testing.T) {
	// Cria um cache com TTL curto e limite baixo (estressa eviction).
	c := NewCache(50*time.Millisecond, 4)
	// Cria um WaitGroup para sincronizar todas as goroutines.
	var wg sync.WaitGroup
	// Lança 50 goroutines concorrentes de escrita/leitura.
	for i := 0; i < 50; i++ {
		wg.Add(1) // Registra uma goroutine no WaitGroup
		// Inicia a goroutine de trabalho com o índice i.
		go func(i int) {
			defer wg.Done()                       // Sinaliza a conclusão da goroutine
			key := string(rune('a' + i%5))        // Chave limitada a 5 valores
			c.Set(key, newTestResponse(200, "x")) // Escreve no cache
			_, _ = c.Get(key)                     // Lê do cache (ignora o retorno)
			c.Len()                               // Consulta o tamanho do cache
		}(i) // Passa o índice para a closure
	}
	// Aguarda a conclusão de todas as goroutines.
	wg.Wait()
}
