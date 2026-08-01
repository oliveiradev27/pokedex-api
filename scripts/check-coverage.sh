#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# check-coverage.sh
#
# Verifica se a cobertura total de testes está acima do limite exigido.
# Uso: ./scripts/check-coverage.sh <arquivo-de-perfil> [limite-em-percentual]
# Ex.: ./scripts/check-coverage.sh coverage.out 90
# Saída: exit 0 se a cobertura >= limite; exit 1 caso contrário.
# ---------------------------------------------------------------------------
set -euo pipefail

# Arquivo de perfil de cobertura (obrigatório).
PROFILE="${1:?uso: check-coverage.sh <profile> [limite]}"
# Limite mínimo de cobertura em percentual (padrão: 90).
THRESHOLD="${2:-90}"

# Extrai a linha "total:" do relatório funcional de cobertura.
# Formato esperado: "total:                    (statements)        96.3%"
total_line="$(go tool cover -func="$PROFILE" | grep -E '^total:' | tail -1)"
# Extrai apenas o percentual (ex.: "96.2") da linha do total.
coverage="$(echo "$total_line" | awk '{print $NF}' | tr -d '%')"

# Verifica se o percentual extraído é numérico (evita erro silencioso).
if ! [[ "$coverage" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
  echo "ERRO: não foi possível extrair a cobertura de $PROFILE"
  exit 1
fi

# Compara a cobertura obtida contra o limite exigido.
echo "Cobertura total: ${coverage}% (limite mínimo: ${THRESHOLD}%)"
# Usa awk para comparação de ponto flutuante (portável).
if awk -v c="$coverage" -v t="$THRESHOLD" 'BEGIN { exit !(c >= t) }'; then
  echo "OK: cobertura acima do limite exigido."
  exit 0
fi

# Limite não atingido: instrui o desenvolvedor sobre o que fazer.
echo "FALHA: cobertura abaixo do limite exigido de ${THRESHOLD}%."
echo "Rode 'make coverage-html' e verifique as funções não cobertas."
exit 1
