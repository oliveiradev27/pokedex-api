// Pacote httpserver: handlers.go define os handlers HTTP do serviço:
// o handler de índice, o handler de proxy da PokeAPI e os handlers de
// documentação OpenAPI/Swagger.
package httpserver

import (
	"encoding/json"                // Serialização das respostas JSON
	"log/slog"                     // Log estruturado dos erros do proxy
	"net/http"                     // Interface HTTP padrão do Go
	"pokedex-api/internal/pokeapi" // Interfaces e tipos do contrato PokeAPI
	"strings"                      // Operações de validação de strings
	"time"                         // Timestamp usado no handler de índice
)

// proxyPrefix é o prefixo dos caminhos que o proxy deve interceptar.
// É usado apenas como documentação do padrão de rota no mux.
const proxyPrefix = "/api/v1/"

// proxyResourcePattern é o padrão de rota do proxy no ServeMux do Go 1.22.
// O segmento "path" captura tudo após o prefixo, incluindo barras.
const proxyResourcePattern = "/api/v1/{path...}"

// IndexHandler serve o JSON de boas-vindas com os endpoints disponíveis.
// Ajuda na descoberta da API e expõe a versão do serviço.
type IndexHandler struct {
	version string // Versão do serviço a ser exibida no índice
}

// NewIndexHandler constrói um IndexHandler com a versão informada.
func NewIndexHandler(version string) *IndexHandler {
	// Retorna a instância com a versão fixada.
	return &IndexHandler{version: version}
}

// ServeIndex responde com informações básicas sobre o serviço.
func (h *IndexHandler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	// Escreve a resposta JSON com nome, versão e lista de endpoints.
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "pokedex-api", // Nome fixo do serviço
		"version": h.version,     // Versão obtida da configuração
		"endpoints": []string{ // Lista dos endpoints públicos
			"/health",                 // Verificação de saúde
			"/api/v1/{resource}/{id}", // Proxy da PokeAPI
			"/docs",                   // Documentação Swagger UI
			"/docs/openapi.json",      // Especificação OpenAPI
		},
		"docs":      "https://pokeapi.co/docs/v2",          // Docs oficiais da PokeAPI
		"timestamp": time.Now().UTC().Format(time.RFC3339), // Momento da resposta
	})
}

// ProxyHandler encaminha requisições para a PokeAPI e repassa a resposta.
// O handler trabalha apenas com a interface Forwarder (testável).
type ProxyHandler struct {
	forwarder pokeapi.Forwarder // Encaminhador injetado (client real ou stub)
	logger    *slog.Logger      // Logger usado para registrar falhas
}

// NewProxyHandler constrói um ProxyHandler com as dependências injetadas.
func NewProxyHandler(forwarder pokeapi.Forwarder, logger *slog.Logger) *ProxyHandler {
	// Retorna a instância com o encaminhador e o logger fixados.
	return &ProxyHandler{forwarder: forwarder, logger: logger}
}

// ServeProxy trata requisições ao padrão /api/v1/{path...}.
// Valida o caminho, encaminha para a PokeAPI e repassa o JSON recebido.
func (h *ProxyHandler) ServeProxy(w http.ResponseWriter, r *http.Request) {
	// Extrai o caminho do recurso capturado pelo padrão do mux.
	resourcePath := r.PathValue("path")
	// Valida se o caminho contém apenas caracteres seguros.
	if !validResourcePath(resourcePath) {
		// Responde 400 descrevendo o problema sem processar mais nada.
		writeJSONError(w, http.StatusBadRequest, "caminho de recurso inválido")
		return // Interrompe o processamento do handler.
	}

	// Encaminha o recurso para a PokeAPI com a query original.
	resp, err := h.forwarder.Forward(r.Context(), resourcePath, r.URL.RawQuery)
	// Se o encaminhamento falhar, responde 502 (Bad Gateway).
	if err != nil {
		// Registra a falha no log com o caminho e o erro.
		h.logger.Error("falha ao encaminhar para a PokeAPI", "path", resourcePath, "err", err)
		// Responde com o erro JSON padrão de gateway.
		writeJSONError(w, http.StatusBadGateway, "falha ao contatar a PokeAPI")
		return // Interrompe o processamento após o erro.
	}

	// Define o content type recebido da PokeAPI na resposta local.
	w.Header().Set("Content-Type", resp.ContentType)
	// Repassa o status HTTP recebido da PokeAPI (200, 404, etc.).
	w.WriteHeader(resp.StatusCode)
	// Escreve o corpo (JSON) recebido da PokeAPI para o cliente.
	_, _ = w.Write(resp.Body)
}

// validResourcePath verifica se um caminho usa apenas caracteres seguros.
// Permite letras minúsculas, dígitos, hífen, underscore e barra simples.
// Rejeita "..", "//", pontos e qualquer caractere de escape.
func validResourcePath(path string) bool {
	// Caminho vazio não é um recurso válido.
	if path == "" {
		return false
	}
	// Itera sobre cada runa (caractere) do caminho.
	for _, ch := range path {
		// Verifica se o caractere está na lista de permitidos.
		valid := (ch >= 'a' && ch <= 'z') || // Letras minúsculas
			(ch >= '0' && ch <= '9') || // Dígitos
			ch == '-' || ch == '_' || ch == '/' // Símbolos seguros
		// Se algum caractere não for permitido, o caminho é inválido.
		if !valid {
			return false
		}
	}
	// Rejeita barras duplas, que indicam caminhos mal formados.
	if strings.Contains(path, "//") {
		return false
	}
	// O caminho passou em todas as validações.
	return true
}

// DocsHandler serve a especificação OpenAPI e a interface Swagger UI.
// Os arquivos são embutidos no binário pelo pacote internal/docs.
type DocsHandler struct {
	spec []byte // Especificação OpenAPI embutida
	ui   []byte // Página HTML do Swagger UI embutida
}

// NewDocsHandler constrói um DocsHandler com os bytes embutidos.
func NewDocsHandler(spec, ui []byte) *DocsHandler {
	// Retorna a instância com a spec e a UI fixadas.
	return &DocsHandler{spec: spec, ui: ui}
}

// ServeSpec escreve a especificação OpenAPI em JSON.
func (h *DocsHandler) ServeSpec(w http.ResponseWriter, r *http.Request) {
	// Define o content type correto para um documento JSON.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Escreve os bytes da especificação embutida.
	_, _ = w.Write(h.spec)
}

// ServeUI escreve a página HTML do Swagger UI.
func (h *DocsHandler) ServeUI(w http.ResponseWriter, r *http.Request) {
	// Define o content type correto para uma página HTML.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Escreve os bytes da página do Swagger UI embutida.
	_, _ = w.Write(h.ui)
}

// writeJSON serializa um valor como JSON e escreve na resposta.
// Aplicada por todos os handlers que devolvem JSON estruturado.
func writeJSON(w http.ResponseWriter, code int, v any) {
	// Define o content type JSON para a resposta.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Define o status HTTP da resposta.
	w.WriteHeader(code)
	// Serializa o valor e escreve na resposta (ignora erro de encode).
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONError é um atalho para escrever um erro JSON padronizado.
// O formato é {"error": "<mensagem>"} para facilitar o parsing.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	// Delega para writeJSON com o mapa de erro padrão.
	writeJSON(w, code, map[string]string{"error": msg})
}
