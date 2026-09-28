#!/usr/bin/env bash
set -euo pipefail

API_URL="https://localhost:8099/module/api/log"
API_KEY="bigsecretrandomstring"

MSG="Prueba de log desde bash"
CODE=0

# 1) POST y mostrar respuesta
post_resp=$(curl -sk \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"msg\":\"${MSG}\",\"code\":${CODE}}" \
  -X POST ${API_URL})

echo "POST response:"
echo "$post_resp" | jq .

# 2) GET de la cadena completa y validación JSON
chain_json=$(curl -sk \
  -H "Authorization: Bearer ${API_KEY}" \
  "${API_URL}")
if ! jq --exit-status . >/dev/null 2>&1 <<<"$chain_json"; then
  echo "ERROR: respuesta GET no es JSON válido" >&2
  exit 1
fi


count=$(jq -r 'length' <<<"$chain_json")
echo "Logs recibidos: ${count}"
echo "${chain_json}"


prev="GENESIS"
ok=true

while IFS= read -r row; do
  ts=$(jq -r '.timestamp'      <<<"$row")
  msg=$(jq -r '.msg'            <<<"$row")
  code=$(jq -r '.code // 0'     <<<"$row")
  hash=$(jq -r '.hash'          <<<"$row")
  pHash=$(jq -r '.previous_hash'<<<"$row")
  id=$(jq -r '.id'              <<<"$row")

  str="PreviousHash:${pHash};Timestamp:${ts};Msg:${msg};Code:${code}"
  calc=$(printf '%s' "$str" | sha256sum | cut -d' ' -f1)

  if [[ "$calc" != "$hash" || "$pHash" != "$prev" ]]; then
    echo "Cadena rota en id ${id}"
    ok=false
    break
  fi

  prev="$hash"
done < <(jq -c '.[]' <<<"$chain_json")

# 5) Resultado
if $ok; then
  echo "✅ Log-chain verificada con éxito"
else
  exit 1
fi
