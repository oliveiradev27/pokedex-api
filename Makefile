# ---------------------------------------------------------------------------
# Makefile do projeto pokedex-api
#
# Comandos utilitários para build, execução, testes e lint.
# Uso: make <alvo> (ex.: make run, make test, make coverage)
# ---------------------------------------------------------------------------

# Nome do binário gerado
BINARY := bin/pokedex-api

# Caminho do perfil de cobertura usado pelos alvos de teste
COVERPROFILE := coverage.out

# Limite mínimo de cobertura exigido (configurável via COVERAGE_THRESHOLD)
COVERAGE_THRESHOLD ?= 90

# Porta padrão usada no alvo `run`
PORT ?= 8080

.PHONY: build run test test-race coverage coverage-html lint vet fmt fmt-check check tidy clean

# Compila o binário do servidor em bin/pokedex-api
build:
	go build -o $(BINARY) ./cmd/api

# Executa o servidor localmente na porta configurada (via variável de ambiente)
run:
	PORT=$(PORT) go run ./cmd/api

# Executa todos os testes do projeto
test:
	go test ./... -count=1

# Executa todos os testes com o detector de corrida (race detector)
test-race:
	go test ./... -race -count=1

# Executa os testes e falha se a cobertura total ficar abaixo do limite
coverage:
	go test ./... -count=1 -coverprofile=$(COVERPROFILE)
	./scripts/check-coverage.sh $(COVERPROFILE) $(COVERAGE_THRESHOLD)

# Gera o relatório HTML de cobertura para visualização no navegador
coverage-html:
	go test ./... -count=1 -coverprofile=$(COVERPROFILE)
	go tool cover -html=$(COVERPROFILE) -o coverage.html

# Executa as verificações estáticas do Go (equivalente ao lint)
lint vet:
	go vet ./...

# Corrige automaticamente a formatação de todos os arquivos Go
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

# Verifica se todos os arquivos Go estão formatados corretamente
fmt-check:
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*')); \
	if [ -n "$$out" ]; then \
		echo "Arquivos fora do padrão de formatação:"; echo "$$out"; exit 1; \
	fi

# Atalho para rodar lint + testes + checagem de cobertura
check: fmt-check vet test coverage

# Remove artefatos gerados (binário e perfis de cobertura)
clean:
	rm -rf bin coverage.out coverage.html
