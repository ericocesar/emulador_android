#!/usr/bin/env bash
set -euo pipefail

ENV_FILE=${1:-.env.github}

if [ ! -f "$ENV_FILE" ]; then
  echo "Arquivo $ENV_FILE não encontrado"
  exit 1
fi

# Detecta o repositório atual dinamicamente
if ! REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null); then
  echo "Erro: Não foi possível detectar um repositório GitHub nesta pasta."
  exit 1
fi

# Função para verificar se existem secrets em um escopo
has_secrets() {
  local env_name=${1:-""}
  local count=0
  if [ -n "$env_name" ]; then
    # Verifica se o ambiente existe primeiro
    if gh api "repos/$REPO/environments/$env_name" &>/dev/null; then
      count=$(gh secret list --repo "$REPO" --env "$env_name" --json name -q 'length' 2>/dev/null || echo 0)
    fi
  else
    count=$(gh secret list --repo "$REPO" --json name -q 'length' 2>/dev/null || echo 0)
  fi
  [ "$count" -gt 0 ]
}

# Verifica se há qualquer secret configurada
HAS_ANY_SECRETS=false
if has_secrets "" || has_secrets "develop" || has_secrets "production"; then
  HAS_ANY_SECRETS=true
fi

if [ "$HAS_ANY_SECRETS" = true ]; then
  echo "===================================================="
  echo "Repositório detectado: $REPO"
  echo "===================================================="
  echo "AVISO: Este script irá APAGAR as secrets existentes"
  echo "no repositório e nos ambientes 'develop' e 'production'"
  echo "antes de enviar as novas definições de $ENV_FILE."
  echo "===================================================="
  read -p "Deseja continuar? (s/N): " confirm
  if [[ ! "$confirm" =~ ^[sS]$ ]]; then
    echo "Operação cancelada pelo usuário."
    exit 0
  fi
fi

# Função para apagar todas as secrets de um escopo
clear_secrets() {
  local env_name=${1:-""}
  if [ -n "$env_name" ]; then
    # Só tenta limpar se o ambiente existir
    if gh api "repos/$REPO/environments/$env_name" &>/dev/null; then
      echo "Limpando secrets do ambiente '$env_name'..."
      gh secret list --repo "$REPO" --env "$env_name" --json name -q '.[].name' | while read -r secret; do
        if [ -n "$secret" ]; then
          gh secret delete "$secret" --repo "$REPO" --env "$env_name"
          echo "  - $secret removida"
        fi
      done
    fi
  else
    echo "Limpando secrets globais do repositório..."
    gh secret list --repo "$REPO" --json name -q '.[].name' | while read -r secret; do
      if [ -n "$secret" ]; then
        gh secret delete "$secret" --repo "$REPO"
        echo "  - $secret removida"
      fi
    done
  fi
}

# Limpa tudo se necessário antes de começar
if [ "$HAS_ANY_SECRETS" = true ]; then
  clear_secrets ""
  clear_secrets "develop"
  clear_secrets "production"
fi

# Função para garantir que o ambiente existe e configurar políticas de branch
setup_environment() {
  local env_name=$1
  local branch_pattern=$2

  echo "Configurando ambiente '$env_name'..."
  
  # Cria ou atualiza o ambiente com política de branch customizada
  gh api -X PUT "repos/$REPO/environments/$env_name" \
    -H "Accept: application/vnd.github+json" \
    --input - >/dev/null <<JSON
{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}
JSON

  # Adiciona a política de branch se não existir
  if ! gh api "repos/$REPO/environments/$env_name/deployment-branch-policies" -q '.branch_policies[].name' | grep -qx "$branch_pattern"; then
    echo "Adicionando política de branch '$branch_pattern' para '$env_name'..."
    gh api -X POST "repos/$REPO/environments/$env_name/deployment-branch-policies" \
      -H "Accept: application/vnd.github+json" \
      -f name="$branch_pattern" -f type="branch" >/dev/null
  fi
}

# Garante que os ambientes existam
setup_environment "develop" "develop"
setup_environment "production" "main"

echo "Processando secrets do arquivo $ENV_FILE..."

while IFS= read -r line; do
  [ -z "$line" ] && continue
  [[ "$line" =~ ^# ]] && continue
  [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue
  
  key=${line%%=*}
  value=${line#*=}
  
  # Remove aspas se existirem
  value=$(echo "$value" | sed -e 's/^"//' -e 's/"$//')

  if [[ "$key" == *"_DEV" ]]; then
    clean_key=${key%_DEV}
    echo "Enviando $key para ambiente 'develop' como $clean_key"
    gh secret set "$clean_key" --repo "$REPO" --env "develop" --body "$value"
  elif [[ "$key" == *"_PROD" ]]; then
    clean_key=${key%_PROD}
    echo "Enviando $key para ambiente 'production' as $clean_key"
    gh secret set "$clean_key" --repo "$REPO" --env "production" --body "$value"
  else
    echo "Enviando $key como secret global do repositório"
    gh secret set "$key" --repo "$REPO" --body "$value"
  fi
done < "$ENV_FILE"

echo "Concluído!"
