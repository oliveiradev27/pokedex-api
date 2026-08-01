// Pacote httpserver contém a camada HTTP: roteamento, handlers e
// middlewares. A separação por responsabilidade mantém cada arquivo focado
// e facilita a escrita de testes unitários isolados.
package httpserver

import (
	"context"       // Armazenamento do request ID no contexto da requisição
	"crypto/rand"   // Geração de IDs aleatórios criptograficamente seguros
	"encoding/hex"  // Codificação dos bytes aleatórios em hexadecimal
	"log/slog"      // Log estruturado das requisições e erros
	"net/http"      // Interface HTTP padrão do Go
	"runtime/debug" // Captura da stack para diagnóstico de panics
	"strconv"       // Conversão de timestamp para string (fallback do ID)
	"time"          // Medição de duração das requisições
)

// requestIDHeader é o nome do header HTTP usado para correlacionar logs.
// O mesmo valor é emitido na resposta para facilitar o debugging.
const requestIDHeader = "X-Request-ID"

// ctxKey é o tipo da chave usada no context.Context da requisição.
// Usar um tipo próprio evita colisões com outras chaves do contexto.
type ctxKey string

// requestIDKey é a chave (tipada) que guarda o request ID no contexto.
const requestIDKey ctxKey = "request_id"

// statusRecorder envolve o http.ResponseWriter para capturar o status HTTP.
// É usado pelo middleware de logging para registrar o código final.
type statusRecorder struct {
	http.ResponseWriter     // Embute o writer original (herda a interface)
	status              int // Status HTTP final registrado (padrão 200)
}

// WriteHeader intercepta a escrita do status para guardá-lo no recorder.
// O status só é registrado na primeira chamada (comportamento do HTTP).
func (r *statusRecorder) WriteHeader(code int) {
	// Guarda o código recebido no campo do recorder.
	r.status = code
	// Repassa a escrita do status para o writer original.
	r.ResponseWriter.WriteHeader(code)
}

// newRequestID gera um ID aleatório de 16 caracteres hexadecimais.
// Em falha raríssima do gerador aleatório, usa o timestamp como fallback.
func newRequestID() string {
	// Aloca o buffer de 8 bytes (16 caracteres hex).
	b := make([]byte, 8)
	// Preenche o buffer com bytes aleatórios seguros.
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp em nanossegundos como identificador.
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	// Codifica os bytes em uma string hexadecimal legível.
	return hex.EncodeToString(b)
}

// requestIDFrom extrai o request ID do contexto da requisição.
// Retorna string vazia quando o ID ainda não foi definido.
func requestIDFrom(r *http.Request) string {
	// Lê o valor tipado do contexto da requisição.
	id, _ := r.Context().Value(requestIDKey).(string)
	// Devolve o ID encontrado (ou string vazia se ausente).
	return id
}

// withRequestID garante que toda requisição tenha um ID de correlação.
// Reutiliza o header informado pelo cliente ou gera um novo ID.
func withRequestID(next http.Handler) http.Handler {
	// Retorna um handler que envolve o próximo da cadeia.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Lê o ID informado pelo cliente (se houver) no header.
		id := r.Header.Get(requestIDHeader)
		// Se o cliente não enviou, gera um novo ID aleatório.
		if id == "" {
			id = newRequestID() // Gera um ID fresco
		}
		// Define o ID na resposta para o cliente correlacionar logs.
		w.Header().Set(requestIDHeader, id)
		// Injeta o ID no contexto e repassa para o próximo handler.
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// withLogging registra um log estruturado para cada requisição recebida.
// Inclui método, caminho, status HTTP final e duração da requisição.
func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	// Retorna um handler que envolve o próximo da cadeia.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Marca o instante de início para calcular a duração.
		start := time.Now()
		// Envolve o writer para capturar o status HTTP final.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		// Repassa a requisição para o próximo handler da cadeia.
		next.ServeHTTP(rec, r)
		// Emite o log estruturado com os dados coletados da requisição.
		logger.Info("http_request", // Mensagem fixa do log
			"method", r.Method, // Método HTTP usado
			"path", r.URL.Path, // Caminho requisitado
			"status", rec.status, // Status final capturado
			"duration_ms", time.Since(start).Milliseconds(), // Duração em ms
			"request_id", requestIDFrom(r), // ID de correlação
		)
	})
}

// withRecovery captura panics e converte em respostas HTTP 500.
// Evita que um pânico derrube o processo e preserva o diagnóstico.
func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	// Retorna um handler que envolve o próximo da cadeia.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Instala a função de recuperação executada ao final do handler.
		defer func() {
			// Recupera o valor do panic (nil quando não houve pânico).
			if rec := recover(); rec != nil {
				// Registra o erro e a stack trace no log.
				logger.Error("panic recuperado", // Mensagem fixa do log
					"err", rec, // Valor do panic
					"stack", string(debug.Stack()), // Stack trace completa
					"request_id", requestIDFrom(r), // ID de correlação
				)
				// Escreve uma resposta de erro JSON 500 para o cliente.
				writeJSONError(w, http.StatusInternalServerError, "erro interno do servidor")
			}
		}()
		// Executa o próximo handler normalmente (sem pânico no caminho feliz).
		next.ServeHTTP(w, r)
	})
}

// withCORS adiciona os headers de CORS e trata requisições OPTIONS.
// A origem permitida é definida por configuração (padrão "*").
func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	// Retorna um handler que envolve o próximo da cadeia.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define a origem permitida pelo CORS.
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		// Define os métodos HTTP permitidos nas requisições cross-origin.
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		// Define os headers customizados permitidos pelo navegador.
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		// Se for um preflight OPTIONS, responde 204 sem processar mais nada.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent) // 204 sem corpo
			return                              // Encerra o preflight aqui
		}
		// Para métodos reais, repassa para o próximo handler da cadeia.
		next.ServeHTTP(w, r)
	})
}
