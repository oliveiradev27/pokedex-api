// Pacote httpserver: testes de integração do roteador completo.
// Exercitam o router com stubs e um servidor fake da PokeAPI.
package httpserver

import (
	"net/http"                     // Interface HTTP padrão do Go
	"net/http/httptest"            // Servidor/recorder de teste
	"pokedex-api/internal/pokeapi" // Tipo Response do contrato PokeAPI
	"strings"                      // Verificação de conteúdo nas respostas
	"testing"                      // Framework de testes padrão do Go
)

// testDependencies monta um Dependencies completo com stubs válidos.
// Centraliza a construção para evitar repetição nos testes do router.
func testDependencies() Dependencies {
	// Retorna as dependências com stub, spec e UI de exemplo.
	return Dependencies{
		Logger:      newDiscardLogger(), // Logger silencioso
		Forwarder:   &stubForwarder{resp: &pokeapi.Response{StatusCode: 200, Body: []byte(`{"ok":true}`)}},
		Pinger:      &stubPinger{err: nil},         // Dependência saudável
		Version:     "1.0.0-test",                  // Versão de teste
		CORSOrigin:  "*",                           // Origem aberta
		OpenAPISpec: []byte(`{"openapi":"3.0.3"}`), // Spec mínima
		SwaggerUI:   []byte("<html>ui</html>"),     // UI mínima
	}
}

// TestRouterHealth valida a rota GET /health no router montado.
func TestRouterHealth(t *testing.T) {
	// Constroi o router completo com as dependências de teste.
	router := NewRouter(testDependencies())
	// Cria a requisição GET para o health check.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o router completo (com middlewares).
	router.ServeHTTP(rec, req)
	// O health deve responder 200 no cenário saudável.
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// O header de request ID deve estar presente na resposta.
	if rec.Header().Get(requestIDHeader) == "" {
		t.Error("header X-Request-ID ausente na resposta")
	}
	// O header de CORS deve estar presente na resposta.
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("header CORS ausente na resposta")
	}
}

// TestRouterProxyIntegration valida o proxy com o router completo.
// Usa um stub funcional para exercitar o caminho real do mux.
func TestRouterProxyIntegration(t *testing.T) {
	// Constroi as dependências base do router.
	deps := testDependencies()
	// Substitui o stub do forwarder por um fake funcional.
	deps.Forwarder = &stubForwarder{resp: &pokeapi.Response{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"name":"fake","id":1}`)}}
	// Constroi o router com as dependências ajustadas.
	router := NewRouter(deps)
	// Cria a requisição GET para um recurso do proxy.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pokemon/fake", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o router completo (com middlewares).
	router.ServeHTTP(rec, req)
	// O proxy deve responder 200.
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// O corpo deve ser o JSON repassado do recurso.
	if !strings.Contains(rec.Body.String(), `"name":"fake"`) {
		t.Errorf("corpo inesperado: %s", rec.Body.String())
	}
}

// TestRouterDocs valida as rotas de documentação no router montado.
func TestRouterDocs(t *testing.T) {
	// Constroi o router completo com as dependências de teste.
	router := NewRouter(testDependencies())
	// Cenário 1: a spec OpenAPI é servida na rota /docs/openapi.json.
	reqSpec := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	// Cria o recorder para capturar a spec.
	recSpec := httptest.NewRecorder()
	// Executa o router para a rota da spec.
	router.ServeHTTP(recSpec, reqSpec)
	// A spec deve ser servida com status 200.
	if recSpec.Code != http.StatusOK {
		t.Errorf("spec Code = %d; esperado 200", recSpec.Code)
	}
	// O corpo deve conter o openapi da spec mínima.
	if !strings.Contains(recSpec.Body.String(), "3.0.3") {
		t.Errorf("spec inesperada: %s", recSpec.Body.String())
	}

	// Cenário 2: o Swagger UI é servido na rota /docs.
	reqUI := httptest.NewRequest(http.MethodGet, "/docs", nil)
	// Cria o recorder para capturar a página.
	recUI := httptest.NewRecorder()
	// Executa o router para a rota da UI.
	router.ServeHTTP(recUI, reqUI)
	// A UI deve ser servida com status 200.
	if recUI.Code != http.StatusOK {
		t.Errorf("ui Code = %d; esperado 200", recUI.Code)
	}
	// O corpo deve conter o HTML da UI mínima.
	if !strings.Contains(recUI.Body.String(), "<html>") {
		t.Errorf("ui inesperada: %s", recUI.Body.String())
	}
}

// TestRouterIndex valida a rota raiz (índice) no router montado.
func TestRouterIndex(t *testing.T) {
	// Constroi o router completo com as dependências de teste.
	router := NewRouter(testDependencies())
	// Cria a requisição GET para a raiz.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o router para a rota raiz.
	router.ServeHTTP(rec, req)
	// O índice deve responder 200.
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// O corpo deve conter o nome do serviço.
	if !strings.Contains(rec.Body.String(), "pokedex-api") {
		t.Errorf("índice inesperado: %s", rec.Body.String())
	}
}

// TestRouterNotFound valida a resposta 404 para rotas inexistentes.
func TestRouterNotFound(t *testing.T) {
	// Constroi o router completo com as dependências de teste.
	router := NewRouter(testDependencies())
	// Cria a requisição GET para uma rota inexistente.
	req := httptest.NewRequest(http.MethodGet, "/rota-que-nao-existe", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o router para a rota inexistente.
	router.ServeHTTP(rec, req)
	// Rotas inexistentes devem retornar 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("Code = %d; esperado 404", rec.Code)
	}
}

// TestRouterMethodNotAllowed valida a resposta 405 para método não aceito.
func TestRouterMethodNotAllowed(t *testing.T) {
	// Constroi o router completo com as dependências de teste.
	router := NewRouter(testDependencies())
	// Cria uma requisição POST para uma rota que só aceita GET.
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o router para a requisição POST.
	router.ServeHTTP(rec, req)
	// Método não permitido deve retornar 405.
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Code = %d; esperado 405", rec.Code)
	}
}
