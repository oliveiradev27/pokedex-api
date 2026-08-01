// Pacote pokeapi: o arquivo cache.go implementa um cache TTL simples e
// concorrente em memória. Entradas expiradas são consideradas ausentes, e
// quando o limite é atingido a entrada mais antiga (ou a primeira expirada)
// é removida para dar lugar à nova.
package pokeapi

import (
	"sync" // Mutex de leitura/escrita para acesso concorrente seguro
	"time" // Comparação e adição de tempos para expiração
)

// cacheEntry representa um item armazenado no cache.
// Guarda a resposta completa e o momento exato de expiração.
type cacheEntry struct {
	resp      *Response // Resposta da PokeAPI armazenada
	expiresAt time.Time // Momento em que a entrada se torna inválida
}

// Cache é um cache TTL thread-safe armazenado em memória.
// Usa um map protegido por RWMutex para leituras concorrentes baratas.
type Cache struct {
	mu         sync.RWMutex          // Protege o map de itens contra corridas
	items      map[string]cacheEntry // Map de chave -> entrada
	ttl        time.Duration         // Tempo de vida padrão das entradas
	maxEntries int                   // Limite máximo de entradas simultâneas
	now        func() time.Time      // Fonte de relógio injetável (testes)
}

// NewCache cria um novo cache com TTL e limite de entradas definidos.
// now é fixado em time.Now para uso em produção.
func NewCache(ttl time.Duration, maxEntries int) *Cache {
	// Inicializa o map, o TTL, o limite e a função de relógio.
	return &Cache{
		items:      make(map[string]cacheEntry), // Map vazio inicialmente
		ttl:        ttl,                         // TTL recebido por parâmetro
		maxEntries: maxEntries,                  // Limite recebido por parâmetro
		now:        time.Now,                    // Relógio real por padrão
	}
}

// Get retorna a resposta armazenada para a chave, se ainda for válida.
// Entradas expiradas são removidas de forma lazy (limpeza sob demanda),
// mantendo o cache limpo sem a necessidade de um processo em segundo plano.
func (c *Cache) Get(key string) (*Response, bool) {
	// Adquire a trava de leitura (permite múltiplos leitores).
	c.mu.RLock()
	// Busca a entrada no map pela chave.
	entry, ok := c.items[key]
	// Libera a trava de leitura após a consulta.
	c.mu.RUnlock()
	// Se a chave não existe, retorna ausente.
	if !ok {
		return nil, false
	}
	// Se a entrada ainda está dentro do TTL, devolve a resposta.
	if c.now().Before(entry.expiresAt) {
		return entry.resp, true
	}
	// Entrada expirada encontrada: remove-a de forma lazy.
	c.mu.Lock()
	// Re-checa o estado atual (outra goroutine pode ter a substituído).
	if e, still := c.items[key]; still && !c.now().Before(e.expiresAt) {
		delete(c.items, key) // Remove a entrada expirada
	}
	// Libera a trava de escrita após a limpeza.
	c.mu.Unlock()
	// Devolve miss para a entrada que expirou.
	return nil, false
}

// Set armazena uma resposta no cache com validade de c.ttl.
// Quando o limite é atingido, evict() libera espaço para a nova entrada.
func (c *Cache) Set(key string, resp *Response) {
	// Adquire a trava de escrita exclusiva.
	c.mu.Lock()
	// Garante o destravamento ao final da operação.
	defer c.mu.Unlock()
	// Se a chave ainda não existe e o limite foi atingido, libera espaço.
	if _, exists := c.items[key]; !exists && len(c.items) >= c.maxEntries {
		c.evict() // Remove entradas para caber a nova
	}
	// Armazena a entrada com o timestamp de expiração calculado.
	c.items[key] = cacheEntry{
		resp:      resp,               // Guarda a resposta recebida
		expiresAt: c.now().Add(c.ttl), // Expira após o TTL configurado
	}
}

// Len retorna o número de entradas atualmente no cache.
// Útil para testes e para observabilidade do tamanho do cache.
func (c *Cache) Len() int {
	// Adquire a trava de leitura para uma contagem segura.
	c.mu.RLock()
	// Garante o destravamento ao final.
	defer c.mu.RUnlock()
	// Devolve o tamanho atual do map de itens.
	return len(c.items)
}

// evict remove uma entrada para liberar espaço no cache.
// Prioriza entradas já expiradas; caso não exista nenhuma, remove uma
// entrada qualquer (o map do Go itera em ordem aleatória).
// Pré-condição: a trava de escrita já deve estar adquirida.
func (c *Cache) evict() {
	// Obtém o instante atual para comparar com as expirações.
	now := c.now()
	// Procura pela primeira entrada expirada para remover.
	for k, entry := range c.items {
		// Se a entrada já expirou, é a candidata ideal.
		if !now.Before(entry.expiresAt) {
			delete(c.items, k) // Remove a entrada expirada
			return             // Sai após remover uma única entrada
		}
	}
	// Nenhuma expirada: remove a primeira entrada do map (arbitrária).
	for k := range c.items {
		delete(c.items, k) // Remove uma entrada para abrir espaço
		return             // Sai após remover uma única entrada
	}
}
