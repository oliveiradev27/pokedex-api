// Pacote docs: testes dos arquivos estáticos embutidos no binário.
package docs

import (
	"encoding/json" // Validação do JSON da especificação
	"strings"       // Verificação de conteúdo nos bytes embutidos
	"testing"       // Framework de testes padrão do Go
)

// TestOpenAPISpecNonEmpty valida que a especificação embutida existe.
func TestOpenAPISpecNonEmpty(t *testing.T) {
	// A especificação não pode estar vazia no binário.
	if len(OpenAPISpec) == 0 {
		t.Fatal("OpenAPISpec está vazio")
	}
}

// TestOpenAPISpecValidJSON valida que a especificação é um JSON válido.
// Decodifica o documento completo e verifica a estrutura mínima.
func TestOpenAPISpecValidJSON(t *testing.T) {
	// Define um struct genérico para validar o JSON embutido.
	var doc map[string]any
	// Decodifica os bytes embutidos como um mapa genérico.
	if err := json.Unmarshal(OpenAPISpec, &doc); err != nil {
		t.Fatalf("OpenAPISpec não é JSON válido: %v", err)
	}
	// A chave openapi deve indicar a versão 3.0.
	if doc["openapi"] != "3.0.3" {
		t.Errorf("openapi = %v; esperado 3.0.3", doc["openapi"])
	}
	// O documento deve declarar paths (rotas) na estrutura.
	if _, ok := doc["paths"]; !ok {
		t.Error("chave paths ausente na especificação")
	}
	// O documento deve declarar components com schemas.
	if _, ok := doc["components"]; !ok {
		t.Error("chave components ausente na especificação")
	}
}

// TestOpenAPISpecHasHealthPath valida a presença da rota de health.
// A documentação deve cobrir o endpoint /health do serviço.
func TestOpenAPISpecHasHealthPath(t *testing.T) {
	// Converte a especificação em string para busca de substrings.
	spec := string(OpenAPISpec)
	// A rota /health deve estar documentada.
	if !strings.Contains(spec, `"/health"`) {
		t.Error("rota /health ausente na especificação")
	}
	// A rota do proxy deve estar documentada.
	if !strings.Contains(spec, `"/api/v1/{resource}/{id}"`) {
		t.Error("rota /api/v1/{resource}/{id} ausente na especificação")
	}
	if !strings.Contains(spec, `"Pokemon"`) {
		t.Error("schema Pokemon ausente na especificação")
	}
	if !strings.Contains(spec, `"Berry"`) || !strings.Contains(spec, `"Item"`) {
		t.Error("schemas Berry/Item ausentes na especificação")
	}
	if !strings.Contains(spec, "curl http://localhost:8080/api/v1/pokemon/pikachu") {
		t.Error("exemplo de request ausente")
	}
}

// TestSpecVersion valida o acesso à versão declarada da especificação.
func TestSpecVersion(t *testing.T) {
	// Obtém a versão declarada na especificação.
	version, err := SpecVersion()
	// Falha o teste se a versão não puder ser extraída.
	if err != nil {
		t.Fatalf("SpecVersion() retornou erro: %v", err)
	}
	// A versão deve ser 1.1.0 conforme o documento.
	if version != "1.1.0" {
		t.Errorf("version = %q; esperado 1.1.0", version)
	}
}

// TestSwaggerUINonEmpty valida que a página embutida existe.
func TestSwaggerUINonEmpty(t *testing.T) {
	// A página não pode estar vazia no binário.
	if len(SwaggerUI) == 0 {
		t.Fatal("SwaggerUI está vazio")
	}
	// A página deve conter o container do Swagger UI.
	if !strings.Contains(string(SwaggerUI), "swagger-ui") {
		t.Error("conteúdo do Swagger UI ausente na página")
	}
}
