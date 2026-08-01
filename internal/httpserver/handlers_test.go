// Pacote httpserver: testes dos handlers de índice, proxy e documentação.
package httpserver

import (
	"context"                      // Tipo usado na assinatura da interface Forwarder
	"encoding/json"                // Decodificação das respostas JSON
	"net/http"                     // Interface HTTP padrão do Go
	"net/http/httptest"            // Servidor/recorder de teste
	"pokedex-api/internal/pokeapi" // Tipo Response do contrato PokeAPI
	"strings"                      // Verificação de conteúdo nas respostas
	"testing"                      // Framework de testes padrão do Go
)

// stubForwarder é um fake da interface Forwarder para os testes do proxy.
// Permite configurar resposta e erro, e capturar os argumentos recebidos.
type stubForwarder struct {
	resp     *pokeapi.Response // Resposta fake a ser devolvida
	err      error             // Erro fake a ser devolvido
	calls    int               // Contador de chamadas ao Forward
	gotPath  string            // Último caminho recebido
	gotQuery string            // Última query recebida
}

// Forward implementa a interface Forwarder com os valores fake configurados.
// Registra os argumentos recebidos e devolve resposta/erro pré-definidos.
func (s *stubForwarder) Forward(_ context.Context, resourcePath, rawQuery string) (*pokeapi.Response, error) {
	// Incrementa o contador de chamadas ao stub.
	s.calls++
	// Registra o caminho do recurso recebido.
	s.gotPath = resourcePath
	// Registra a query string recebida.
	s.gotQuery = rawQuery
	// Devolve a resposta fake e o erro fake configurados.
	return s.resp, s.err
}

// TestProxyHandlerValid valida o repasse de uma resposta bem-sucedida.
func TestProxyHandlerValid(t *testing.T) {
	// Cria a resposta fake que o proxy deve repassar.
	resp := &pokeapi.Response{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"id":25}`)}
	// Cria o stub do forwarder com a resposta fake configurada.
	forwarder := &stubForwarder{resp: resp}
	// Instancia o handler de proxy com o stub e um logger mudo.
	h := NewProxyHandler(forwarder, newDiscardLogger())
	// Cria a requisição para um recurso válido.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pokemon/pikachu", nil)
	// Registra o caminho capturado pelo padrão do mux.
	req.SetPathValue("path", "pokemon/pikachu")
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o handler do proxy.
	h.ServeProxy(rec, req)
	// O status deve ser repassado da resposta fake.
	if rec.Code != 200 {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// O content type deve ser repassado da resposta fake.
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; esperado application/json", ct)
	}
	// O corpo deve ser repassado integralmente.
	if rec.Body.String() != `{"id":25}` {
		t.Errorf("Body = %q; esperado {\"id\":25}", rec.Body.String())
	}
	// O caminho deve chegar correto ao forwarder.
	if forwarder.gotPath != "pokemon/pikachu" {
		t.Errorf("path repassado = %q", forwarder.gotPath)
	}
}

// TestProxyHandlerInvalidPath valida a rejeição de caminhos inseguros.
// Usa uma tabela de caminhos maliciosos para exercitar a validação.
func TestProxyHandlerInvalidPath(t *testing.T) {
	// Cria o stub do forwarder (não deve ser chamado).
	forwarder := &stubForwarder{}
	// Instancia o handler de proxy com o stub e um logger mudo.
	h := NewProxyHandler(forwarder, newDiscardLogger())
	// Lista de caminhos inválidos a serem testados.
	invalidPaths := []string{
		"",                   // Caminho vazio
		"../etc/passwd",      // Traversal de diretório
		"pokemon/1/../admin", // Traversal dentro do caminho
		"pokemon//1",         // Barra dupla
		"pokemon;id=1",       // Injeção de query
		"pokemon?x=1",        // Query no caminho
		"Pokemon/pikachu",    // Letras maiúsculas (fora do padrão)
		"pokemon/pikachu%20", // Percent-encoding (escape)
	}
	// Itera sobre cada caminho inválido da tabela.
	for _, path := range invalidPaths {
		// Cria a requisição para o caminho inválido atual.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/"+path, nil)
		// Registra o caminho inválido no PathValue.
		req.SetPathValue("path", path)
		// Cria o recorder para capturar a resposta.
		rec := httptest.NewRecorder()
		// Executa o handler do proxy.
		h.ServeProxy(rec, req)
		// O caminho inválido deve ser rejeitado com 400.
		if rec.Code != http.StatusBadRequest {
			t.Errorf("caminho %q: Code = %d; esperado 400", path, rec.Code)
		}
	}
	// O forwarder não deve ter sido chamado em nenhum caso.
	if forwarder.calls != 0 {
		t.Errorf("forwarder chamado %d vezes; esperado 0", forwarder.calls)
	}
}

// TestProxyHandlerUpstreamError valida a resposta 502 em falha de upstream.
func TestProxyHandlerUpstreamError(t *testing.T) {
	// Cria o stub que devolve um erro de rede.
	forwarder := &stubForwarder{err: http.ErrServerClosed}
	// Instancia o handler de proxy com o stub e um logger mudo.
	h := NewProxyHandler(forwarder, newDiscardLogger())
	// Cria a requisição para um recurso válido.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pokemon/1", nil)
	// Registra o caminho capturado pelo padrão do mux.
	req.SetPathValue("path", "pokemon/1")
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o handler do proxy.
	h.ServeProxy(rec, req)
	// A falha de upstream deve resultar em 502 Bad Gateway.
	if rec.Code != http.StatusBadGateway {
		t.Errorf("Code = %d; esperado 502", rec.Code)
	}
	// O corpo deve ser JSON de erro válido.
	var body map[string]string
	// Decodifica o corpo da resposta capturada.
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// A mensagem de erro deve estar presente no corpo.
	if body["error"] == "" {
		t.Error("mensagem de erro ausente no corpo 502")
	}
}

// TestIndexHandler valida o JSON de boas-vindas do índice.
func TestIndexHandler(t *testing.T) {
	// Instancia o handler de índice com versão fixa.
	h := NewIndexHandler("9.9.9")
	// Cria a requisição GET para a raiz.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Cria o recorder para capturar a resposta.
	rec := httptest.NewRecorder()
	// Executa o handler de índice.
	h.ServeIndex(rec, req)
	// O índice deve responder 200.
	if rec.Code != http.StatusOK {
		t.Errorf("Code = %d; esperado 200", rec.Code)
	}
	// Decodifica o corpo do índice para inspeção.
	var body map[string]any
	// Decodifica o corpo da resposta capturada.
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	// O nome do serviço deve estar presente.
	if body["name"] != "pokedex-api" {
		t.Errorf("name = %v; esperado pokedex-api", body["name"])
	}
	// A versão deve ser a configurada.
	if body["version"] != "9.9.9" {
		t.Errorf("version = %v; esperado 9.9.9", body["version"])
	}
}

// TestDocsHandlers valida o servimento da spec OpenAPI e do Swagger UI.
func TestDocsHandlers(t *testing.T) {
	// Cria o handler de docs com bytes de exemplo.
	h := NewDocsHandler([]byte(`{"openapi":"3.0.3"}`), []byte("<html>swagger</html>"))
	// Cenário 1: a rota da especificação serve o JSON.
	reqSpec := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	// Cria o recorder para capturar a spec.
	recSpec := httptest.NewRecorder()
	// Executa o handler de especificação.
	h.ServeSpec(recSpec, reqSpec)
	// O content type da spec deve ser JSON.
	if ct := recSpec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q; esperado JSON", ct)
	}
	// O corpo deve ser a especificação embutida.
	if recSpec.Body.String() != `{"openapi":"3.0.3"}` {
		t.Error("especificação não foi servida corretamente")
	}

	// Cenário 2: a rota da UI serve a página HTML.
	reqUI := httptest.NewRequest(http.MethodGet, "/docs", nil)
	// Cria o recorder para capturar a página.
	recUI := httptest.NewRecorder()
	// Executa o handler da interface.
	h.ServeUI(recUI, reqUI)
	// O content type da UI deve ser HTML.
	if ct := recUI.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q; esperado HTML", ct)
	}
	// O corpo deve ser a página embutida.
	if recUI.Body.String() != "<html>swagger</html>" {
		t.Error("página Swagger UI não foi servida corretamente")
	}
}
