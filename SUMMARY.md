# SUMMARY — Diário de Bordo do Agente

> **Propósito**: este arquivo é otimizado para agentes de IA (e revisores técnicos).
> Ele registra o contexto, as decisões de implementação, os padrões usados e o estado
> atual do trabalho. **Leia-o antes de modificar qualquer código.** Para documentação
> humana (diagramas, setup, execução), consulte o `CHANGELOG.md`.

---

## 1. Visão Geral

- **Projeto**: `pokedex-api` — API proxy em Go que consome a [PokeAPI](https://pokeapi.co/)
  e repassa os JSONs recebidos, com endpoint de health check e documentação OpenAPI/Swagger.
- **Linguagem**: Go 1.26.4 (stdlib apenas — **zero dependências externas**).
- **Módulo**: `pokedex-api`.
- **Versão**: 1.1.0 (bump após auditoria de código do agente `da2b3005`).
- **Estado**: implementado, testado, cobertura **96.3%** (alvo: >90%). Sem corridas
  de dados (`-race` limpo). `go vet` e `gofmt` limpos.
- **Comentários**: todo código Go é comentado **linha a linha em português**.

---

## 2. Mapa de Arquivos

| Caminho | Responsabilidade | Cobertura |
|---|---|---|
| `cmd/api/main.go` | Entry point (enxuto por convenção); `run()` orquestra; `newLogger()` | run/newLogger 100% (main 0%: não testável por `os.Exit`) |
| `cmd/api/main_test.go` | Testes do entry point, incluindo ciclo de vida real via SIGTERM | — |
| `internal/config/config.go` | Leitura de env vars com defaults e validação | 100% |
| `internal/pokeapi/client.go` | Cliente HTTP da PokeAPI: `Forward` (proxy com retentativas) e `Ping` | 95.6% |
| `internal/pokeapi/cache.go` | Cache TTL thread-safe em memória (RWMutex) | — |
| `internal/httpserver/router.go` | Monta o `ServeMux` (Go 1.22 patterns) + cadeia de middlewares | 98.9% |
| `internal/httpserver/handlers.go` | Handlers de índice, proxy (`/api/v1/{path...}`) e docs | — |
| `internal/httpserver/health.go` | Handler `GET /health` com deep check da PokeAPI | — |
| `internal/httpserver/middleware.go` | `withCORS`, `withRequestID`, `withLogging`, `withRecovery` | — |
| `internal/server/server.go` | Composição (config+client+cache+router) e ciclo de vida/graceful shutdown | 93.1% |
| `internal/docs/docs.go` | `//go:embed` do `openapi.json` e do Swagger UI | 75% (embed + helper) |
| `internal/docs/openapi.json` | Especificação OpenAPI 3.0.3 (fonte única da doc) | — |
| `internal/docs/index.html` | Página Swagger UI (CDN) que lê `/docs/openapi.json` | — |
| `Makefile` | Alvos `build/run/test/coverage/lint/fmt/check/clean` | — |
| `scripts/check-coverage.sh` | Falha se cobertura total < limite (padrão 90%) | — |
| `CHANGELOG.md` | Documentação humana (diagramas mermaid, tabelas, setup) | — |

---

## 3. Decisões de Implementação (Decision Log)

### D1. Zero dependências externas (stdlib pura)
- **Decisão**: usar apenas `net/http` (com o roteamento por padrões do Go 1.22+), `log/slog`,
  `//go:embed` e o restante da stdlib. Sem gin/chi, sem viper, sem go-swagger.
- **Motivo**: projeto pequeno e self-contained; elimina supply-chain, facilita build offline
  e torna a cobertura/behavior totalmente previsível.
- **Alternativas rejeitadas**: `chi` (router), `gin` (framework), `swag` (geração de swagger
  por anotações — exigiria CLI e anotações propagadas no código), `redis`/`groupcache` (cache).

### D2. Interfaces no produtor (`internal/pokeapi`), consumo no `httpserver`
- **Decisão**: `Forwarder`, `Pinger` e `Response` vivem no pacote `pokeapi`; o
  `httpserver` os importa. Inicialmente as interfaces foram definidas no lado consumidor
  (padrão "consumer-side interfaces"), mas isso obrigaria `pokeapi` a importar `httpserver`
  apenas pelo tipo `Response` — acoplamento inverso. **Revertido**: contrato no produtor.
- **Consequência**: `internal/httpserver` depende de `internal/pokeapi` (sem ciclo:
  `pokeapi` só importa stdlib). Stubs de teste implementam o contrato por duck typing.

### D3. `ServeMux` do Go 1.22+ com padrões por método
- Rotas: `GET /health`, `GET /api/v1/{path...}` (wildcard), `GET /docs`,
  `GET /docs/openapi.json`, `GET /{$}`.
- `GET /{$}` (em vez de `GET /`) **deliberado**: evita que QUALQUER GET desconhecido
  caia no índice e retorne 200. Rotas desconhecidas → 404; método errado em rota
  conhecida → 405 (comportamento nativo do `ServeMux`).

### D4. Repasse de status: proxy não mascara erros da origem
- 4xx (ex.: 404) da PokeAPI são **repassados com o mesmo status e corpo**.
- 5xx são retentados até `MAX_RETRIES`; esgotadas as tentativas, o **status final é
  repassado como resposta** (o proxy continua transparente).
- Apenas **erros de rede** (conexão recusada, timeout, DNS) viram **502 Bad Gateway**.
- **Alternativa rejeitada**: converter qualquer 5xx da origem em 502 — menos transparente
  para quem consome o proxy e esconde o diagnóstico real da PokeAPI.

### D5. Cache TTL em memória com RWMutex
- Chave = `resourcePath + "?" + rawQuery` (quando há query). TTL e limite configuráveis.
- Expiração por relógio injetável (`now func() time.Time`) — permite testes determinísticos
  sem `sleep`.
- Eviction: prioriza entradas expiradas; sem expiradas, remove entrada arbitrária
  (iteração do map é não determinística — aceito para cache).
- Não há invalidate/negative-cache: apenas 2xx são cacheados (4xx/5xx nunca entram no cache).

### D6. Health check com deep check da dependência
- `GET /health`: 200 `ok` quando a PokeAPI responde 2xx; 503 `degraded` quando não.
- Timeout de 5s no ping; corpo expõe `status`, `version`, `uptime_seconds`, `dependencies.pokeapi`, `timestamp`.
- **Alternativa rejeitada**: liveness/readiness separados (`/health/live` + `/health/ready`) —
  overkill para um serviço com uma única dependência.

### D7. Validação de path do proxy (defesa em profundidade)
- `validResourcePath` aceita apenas `[a-z0-9-_/]`, rejeita `..`, `//`, maiúsculas,
  percent-encoding e caracteres de escape. Caminhos inválidos → 400.
- O `net/http` do Go **já** limpa/redireciona (307) paths com `..` ou `//` antes do handler;
  a validação é a camada extra que protege chamadas diretas ao handler (testada em unidade).

### D8. `Response` em bytes (não streaming)
- O cliente lê o corpo inteiro (limitado a 10 MiB) e devolve `[]byte`. Permite cache
  simples e testes determinísticos. Trade-off aceito: corpo em memória.

### D9. Retentativas com backoff linear simples
- Laço `0..maxRetries`, backoff `100ms * tentativa`, cancelável via `ctx.Done()`.
- **Alternativa rejeitada**: exponential backoff com jitter/retry budgets — complexidade
  desnecessária para uma API pública de baixa frequência.

### D10. Graceful shutdown com listener injetável
- `server.Run(ctx, cfg, logger, opts...)` aceita `WithListener` (testes usam `:0`).
- `Run` bloqueia até `ctx.Done()` ou erro do `Serve`; faz `Shutdown` com timeout de 10s.
- `main` usa `signal.NotifyContext(SIGINT, SIGTERM)`.

### D11. Testes de ciclo de vida reais
- `server_test`: sobe servidor real em porta efêmera, valida proxy (502 quando base da
  PokeAPI inacessível) e encerramento gracioso.
- `main_test`: sobe o processo completo e envia **SIGTERM para o próprio processo** para
  exercitar o `NotifyContext` real (é seguro: o servidor só fica de pé após o `NotifyContext`
  estar ativo).

### D12. Backoff exponencial com jitter (após auditoria)
- Substituiu o backoff linear fixo (D9 original). Delay = `base * 2^(attempt-1)` com teto
  de 2s e jitter aleatório de 0 a `base` (100ms), usando `math/rand/v2` (seguro para uso
  concorrente). O jitter evita "ondas" sincronizadas de retentativas contra a PokeAPI.
- **Achado de auditoria (severidade média)** do agente `da2b3005`.

### D13. Remoção lazy de entradas expiradas no cache (após auditoria)
- `Cache.Get` agora **remove** entradas expiradas ao encontrá-las (limpeza sob demanda),
  em vez de apenas tratá-las como miss. Usa double-check com trava de escrita para
  evitar corridas com `Set` concorrente. Elimina o acúmulo de "lixo" expirado.
- **Achado de auditoria (severidade média)** do agente `da2b3005`.

### D14. Log de cache miss (após auditoria)
- `Client.Forward` agora registra `Debug("cache miss", ...)` além do `cache hit`,
  para observabilidade da taxa de acerto e do tráfego real à PokeAPI.
- **Achado de auditoria (severidade baixa)** do agente `da2b3005`.

---

## 3-A. Auditoria de Código (agente `da2b3005`)

- **Papel**: o agente "Configuração de Auditoria de Código" (deepseeker/openrouter) auditou
  o trabalho do executor. Revisou: SUMMARY/CHANGELOG, middleware, handlers, router, client,
  cache, server e config.
- **Achados e resolução**:

| Severidade | Achado | Resolução |
|---|---|---|
| Alta | CORS padrão `*` é permissivo em produção | **Aceito como decisão**: é env-configurável (`CORS_ALLOWED_ORIGIN`); produção deve fixar origem específica. Sem mudança de código. |
| Média | Backoff linear → exponencial com jitter | **Corrigido** (D12): `retryDelay()` + `math/rand/v2`. |
| Média | `Get` não removia expirados (acúmulo) | **Corrigido** (D13): remoção lazy com double-check. |
| Baixa | Faltava log de cache miss | **Corrigido** (D14): `Debug("cache miss", ...)`. |
| Baixa | `validResourcePath` rejeita chars fora de `a-z` | **Aceito como decisão**: recursos da PokeAPI usam lowercase; validação mais estrita é defesa em profundidade (D7). |
- **Validação pós-ajustes**: `go vet` limpo, `-race` limpo, testes verdes, cobertura
  subiu para **96.3%**.

## 4. Conhecimentos de Uso Frequente

### Comandos
```bash
make test           # testes com -count=1
make coverage       # testes + falha se < 90% (check-coverage.sh)
make coverage-html  # relatório visual em coverage.html
make test-race      # detector de corrida
make vet            # go vet
make run            # PORT=8080 go run ./cmd/api
make check          # fmt-check + vet + test + coverage
```

### Cobertura atual (go test -cover)
| Pacote | Cobertura |
|---|---|
| internal/config | 100.0% |
| internal/httpserver | 98.9% |
| internal/pokeapi | 95.6% |
| internal/server | 93.1% |
| cmd/api | 83.3% (main não testável) |
| internal/docs | 75.0% (embed + `SpecVersion`) |
| **Total** | **96.3%** |

### Env vars (ver `CHANGELOG.md` para detalhes)
`PORT`, `POKEAPI_BASE_URL`, `REQUEST_TIMEOUT_MS`, `CACHE_ENABLED`, `CACHE_TTL_SECONDS`,
`CACHE_MAX_ENTRIES`, `MAX_RETRIES`, `LOG_LEVEL`, `LOG_FORMAT`, `CORS_ALLOWED_ORIGIN`, `VERSION`.

---

## 5. Gotchas e Armadilhas (para agentes futuros)

1. **`main()` a 0% de cobertura é INTENCIONAL** — é impossível testar um `os.Exit` sem
   matar o binário de teste. Não tente "corrigir" cobrindo `main`; o alvo de 90% é sobre o
   total, e `run()`/`newLogger()` já são 100% cobertos.
2. **`SpecVersion` 75%**: o branch de erro (`json.Unmarshal` falho) não é coberto porque o
   `OpenAPISpec` embutido é sempre válido. Não vale a pena "quebrar" o embed para cobrir.
3. **Porta em conflito no teste**: reservar `127.0.0.1:0` e bindar depois em `:port` NÃO
   conflita no macOS (SO_REUSEADDR permite). Para conflito real, reserve em `:0` (todas as
   interfaces).
4. **`waitHealthy` em `server_test`** aceita qualquer status HTTP (o health pode ser 503
   quando a base da PokeAPI é um endereço inacessível nos testes).
5. **Cache debug**: logs de `cache hit`/`cache miss` são `Debug` — não aparecem com
   `LOG_LEVEL` padrão (INFO).
6. **`.gitignore`** cobre `bin/`, `coverage.out`, `.env`. O projeto ainda **não é um repo git**
   (a diretoria foi inicializada vazia).
7. **307 redirects do net/http**: paths com `..`/`//` são redirecionados pelo próprio Go
   antes do handler — não "consertar" assumindo que a validação de path é a única defesa.
8. **Cobertura usa `-covermode=atomic`** no script/Makefile; o `Makefile` roda `check-coverage.sh`.

---

## 6. Não-objetivos (Non-goals)

- Autenticação/authorização, rate limiting por cliente, persistência (banco de dados).
- Cache distribuído (Redis) ou cache de disco.
- Enriquecimento/modelagem dos dados (o proxy repassa os JSONs **verbatim**).
- Suporte a POST/PUT/DELETE (API pública da PokeAPI é somente leitura).
- Deploy (Docker/k8s) — fora do escopo inicial; fácil de adicionar depois.
- Múltiplas instâncias/sessão — o cache em memória é por instância.

---

## 7. Sugestões de Evolução (backlog técnico)

- `Dockerfile` + `docker-compose.yml` (multi-stage build).
- Middleware de timeout global por requisição e limitador de corpo de resposta.
- Métricas Prometheus (`/metrics`) com contadores de cache hit/miss e latência upstream.
- Instrumentação com `context` propagando request ID até a chamada upstream.
- Testes de tabela para a spec OpenAPI (validação estrutural mais profunda).
