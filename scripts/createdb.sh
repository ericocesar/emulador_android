#!/bin/bash

# Script para criar novo banco de dados PostgreSQL
# Uso: ./create-database.sh <nome_do_banco> [usuario_postgres]

# Verifica se o nome do banco foi fornecido
if [ $# -eq 0 ]; then
    echo "❌ Erro: Por favor, forneça o nome do banco de dados"
    echo "Uso: $0 <nome_do_banco> [usuario_postgres]"
    echo "Exemplo: $0 meubanco"
    echo "Exemplo: $0 meubanco ericocesar"
    exit 1
fi

DB_NAME=$1
POSTGRES_USER=${2:-"ericocesar"}  # Usa 'ericocesar' (superuser) como padrão
DB_OWNER="app"
DB_USER="app"

# Cores para output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔧 Criando banco de dados PostgreSQL...${NC}"
echo -e "${BLUE}   Nome do banco: ${DB_NAME}${NC}"
echo -e "${BLUE}   Owner: ${DB_OWNER}${NC}"
echo -e "${BLUE}   Usuário PostgreSQL: ${POSTGRES_USER}${NC}"
echo ""

# Comando para criar o banco de dados com owner
CREATE_DB_CMD="CREATE DATABASE ${DB_NAME} OWNER ${DB_OWNER};"

# Comando para conceder privilégios
GRANT_PRIV_CMD="GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

# Executa os comandos PostgreSQL
if psql -h localhost -U "${POSTGRES_USER}" -d postgres -c "${CREATE_DB_CMD}"; then
    echo -e "${GREEN}✅ Banco de dados '${DB_NAME}' criado com sucesso!${NC}"
    
    if psql -h localhost -U "${POSTGRES_USER}" -d postgres -c "${GRANT_PRIV_CMD}"; then
        echo -e "${GREEN}✅ Privilégios concedidos ao usuário '${DB_USER}'!${NC}"
        echo ""
        echo -e "${GREEN}🎉 Banco de dados criado e configurado com sucesso!${NC}"
        echo ""
        echo -e "${BLUE}📋 Configuração da DATABASE_URL:${NC}"
        echo -e "${GREEN}DATABASE_URL=postgresql://app:app@localhost:5432/${DB_NAME}${NC}"
        echo ""
        echo -e "${BLUE}💡 Adicione esta variável ao seu arquivo .env:${NC}"
        echo ""
    else
        echo -e "${RED}❌ Erro ao conceder privilégios ao banco de dados${NC}"
        exit 1
    fi
else
    echo -e "${RED}❌ Erro ao criar banco de dados${NC}"
    exit 1
fi