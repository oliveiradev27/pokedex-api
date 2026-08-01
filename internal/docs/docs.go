// Pacote docs embute os arquivos estáticos de documentação no binário.
// Usando //go:embed, a especificação OpenAPI e a página do Swagger UI
// ficam disponíveis em tempo de execução sem dependência de arquivos
// externos ao binário compilado.
package docs

import (
	_ "embed"       // Habilita a diretiva de embutimento de arquivos
	"encoding/json" // Validação da especificação nos testes
)

// OpenAPISpec contém a especificação OpenAPI 3.0 em formato JSON.
// Serve ao Swagger UI e aos consumidores que precisam do schema.
//
//go:embed openapi.json
var OpenAPISpec []byte

// SwaggerUI contém a página HTML que carrega o Swagger UI.
// A página lê a especificação em /docs/openapi.json em tempo de execução.
//
//go:embed index.html
var SwaggerUI []byte

// SpecVersion retorna a versão da especificação OpenAPI declarada.
// Útil para logs de inicialização e para testes de consistência.
func SpecVersion() (string, error) {
	// Define um struct mínimo apenas para extrair a versão.
	var doc struct {
		Info struct { // Bloco de informações da especificação
			Version string // Campo version do info
		} `json:"info"` // Tag do campo info
	} // Fecha a definição do struct local
	// Decodifica os bytes embutidos no struct local.
	if err := json.Unmarshal(OpenAPISpec, &doc); err != nil {
		return "", err // Propaga erro de decodificação
	}
	// Devolve a versão declarada na especificação.
	return doc.Info.Version, nil
}
