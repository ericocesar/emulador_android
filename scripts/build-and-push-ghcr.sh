#!/usr/bin/env bash
set -euo pipefail

# Build and Push para GitHub Container Registry (ghcr.io)
# Uso: ./scripts/build-and-push-ghcr.sh [--no-cache]
#
# Gera duas imagens:
#   - ghcr.io/<namespace>/<projeto>-gateway:<tag>
#   - ghcr.io/<namespace>/<projeto>-frontend:<tag>
#
# Tags aplicadas:
#   - sha-<7chars>   (ex: sha-abc1234)
#   - <branch>        (ex: main, develop)
#
# Flags:
#   --no-cache   Build SEM cache do Docker (padrão: usa cache)
#
# Requisitos:
#   - Docker rodando com buildx
#   - Login no ghcr.io (echo "$GHCR_TOKEN" | docker login ghcr.io -u USER --password-stdin)
#   - Trivy instalado (brew install aquasecurity/trivy/trivy)

REGISTRY="${REGISTRY:-ghcr.io}"
PLATFORM="linux/amd64"

NO_CACHE=false
for arg in "$@"; do
  if [ "$arg" == "--no-cache" ]; then
    NO_CACHE=true
  fi
done

sanitize_component() {
  echo "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's#^[^a-z0-9]+##; s#[^a-z0-9._-]+#-#g; s#-+#-#g; s#[-._]+$##'
}

detect_project_name() {
  local name=""
  local root_dir="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

  if [[ -f "${root_dir}/package.json" ]]; then
    name="$(sed -nE 's/^[[:space:]]*"name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "${root_dir}/package.json" | head -n1)"
  fi

  if [[ -z "${name}" ]]; then
    name="$(basename "${root_dir}")"
  fi

  name="${name##*/}"
  sanitize_component "${name}"
}

detect_namespace() {
  local namespace=""

  if [[ -n "${IMAGE_NAMESPACE:-}" ]]; then
    namespace="${IMAGE_NAMESPACE}"
  elif [[ -n "${GITHUB_REPOSITORY_OWNER:-}" ]]; then
    namespace="${GITHUB_REPOSITORY_OWNER}"
  elif [[ -n "${GITHUB_REPOSITORY:-}" ]]; then
    namespace="${GITHUB_REPOSITORY%%/*}"
  else
    local remote_url="$(git config --get remote.origin.url 2>/dev/null || true)"
    if [[ -n "${remote_url}" ]]; then
      namespace="$(echo "${remote_url}" | sed -E 's#(git@|https?://|ssh://git@)?[^/:]+[:/]([^/]+)/.*#\2#')"
    fi
  fi

  if [[ -z "${namespace}" ]]; then
    namespace="ericocesar"
  fi

  sanitize_component "${namespace}"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

PROJECT_NAME_RESOLVED="$(detect_project_name)"
IMAGE_NAMESPACE_RESOLVED="$(detect_namespace)"

# Nome do repositório no registry. Override opcional via env var:
#   IMAGE_GATEWAY_NAME=meu-gateway ./scripts/build-and-push-ghcr.sh
# Default: <project>-gateway / <project>-frontend
IMAGE_GATEWAY_NAME="${IMAGE_GATEWAY_NAME:-${PROJECT_NAME_RESOLVED}-gateway}"
IMAGE_FRONTEND_NAME="${IMAGE_FRONTEND_NAME:-${PROJECT_NAME_RESOLVED}-frontend}"

IMAGE_GATEWAY="${REGISTRY}/${IMAGE_NAMESPACE_RESOLVED}/${IMAGE_GATEWAY_NAME}"
IMAGE_FRONTEND="${REGISTRY}/${IMAGE_NAMESPACE_RESOLVED}/${IMAGE_FRONTEND_NAME}"

GIT_SHA_SHORT="$(git rev-parse --short=7 HEAD)"
BRANCH="$(git rev-parse --abbrev-ref HEAD | tr '[:upper:]' '[:lower:]' | tr '/' '-')"
TAG_SHA="sha-${GIT_SHA_SHORT}"
TAG_BRANCH="${BRANCH}"

echo "============================================="
echo "  Build and Push Docker - GHCR"
echo "============================================="
echo "Registry:    ${REGISTRY}"
echo "Namespace:   ${IMAGE_NAMESPACE_RESOLVED}"
echo "Project:     ${PROJECT_NAME_RESOLVED}"
echo "Platform:    ${PLATFORM}"
echo "Branch:      ${BRANCH}"
echo "Tags:        ${TAG_BRANCH}, ${TAG_SHA}"
if [ "${NO_CACHE}" = true ]; then
  echo "Cache:       DESABILITADO (--no-cache)"
else
  echo "Cache:       habilitado"
fi
echo ""
echo "Imagens:"
echo "  Gateway:   ${IMAGE_GATEWAY}:${TAG_SHA}"
echo "  Frontend:  ${IMAGE_FRONTEND}:${TAG_SHA}"
echo "============================================="

# ---------------------------------------------------
# 0. Commit e push das alterações
# ---------------------------------------------------
echo ""
echo ">>> Verificando status do Git..."

if ! git rev-parse --git-dir > /dev/null 2>&1; then
  echo "  ⚠️  Este diretório não é um repositório Git. Pulando commit/push."
else
  if git diff --quiet && git diff --cached --quiet; then
    echo "    ✓ Não há alterações para commitar."
  else
    echo "    → Fazendo commit das alterações..."

    git add -A

    COMMIT_MSG="chore: build ${PROJECT_NAME_RESOLVED}:${TAG_SHA}"
    git commit -m "${COMMIT_MSG}" || echo "    ⚠️  Nada para commitar (já está sincronizado)"

    echo "    → Fazendo push para o remote..."
    CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
    git push origin "${CURRENT_BRANCH}" 2>/dev/null || git push origin main 2>/dev/null || echo "    ⚠️  Push falhou ou não há remote configurado"

    echo "    ✓ Commit e push concluídos."
  fi

  GIT_SHA_SHORT="$(git rev-parse --short=7 HEAD)"
  TAG_SHA="sha-${GIT_SHA_SHORT}"
fi

BUILD_TZ="${TZ:-America/Recife}"
BUILD_DATE="$(TZ="${BUILD_TZ}" date +%d/%m/%y)"
BUILD_TIME="$(TZ="${BUILD_TZ}" date +%H:%M)"
BUILD_TIMESTAMP="$(TZ="${BUILD_TZ}" date +%Y-%m-%dT%H:%M:%S%z)"
BUILD_COMMIT="${GIT_SHA_SHORT}"

# ---------------------------------------------------
# 1. Verificação de segurança com Trivy (pré-build)
# ---------------------------------------------------
echo ""
echo ">>> Executando varredura de segurança com Trivy..."

if ! command -v trivy &> /dev/null; then
  echo ""
  echo "  ⚠️  Trivy não encontrado. Instale com:"
  echo "     brew install aquasecurity/trivy/trivy"
  echo ""
  echo "  Abortando build por segurança."
  exit 1
fi

TRIVY_EXIT_CODE=0
trivy fs \
  --scanners misconfig,vuln \
  --severity HIGH,CRITICAL \
  --exit-code 1 \
  --skip-version-check \
  --ignorefile .trivyignore \
  . || TRIVY_EXIT_CODE=$?

if [[ "${TRIVY_EXIT_CODE}" -ne 0 ]]; then
  echo ""
  echo "============================================="
  echo "  ❌ Trivy encontrou vulnerabilidades HIGH/CRITICAL!"
  echo "============================================="
  echo ""
  echo "  Por favor, corrija as vulnerabilidades antes de fazer o build."
  echo "  Para ver detalhes completos, execute:"
  echo ""
  echo "     trivy fs --scanners misconfig,vuln --severity HIGH,CRITICAL --ignorefile .trivyignore ."
  echo ""
  echo "  Dicas de correção:"
  echo "    - Dependências npm:  npm update <pacote> ou ajuste a versão em package.json"
  echo "    - Dockerfile:        adicione USER <non-root> no Dockerfile"
  echo ""
  echo "  Para ignorar uma vulnerabilidade específica (caso seja falso positivo),"
  echo "  crie um arquivo .trivyignore na raiz do projeto com os CVE IDs."
  echo "============================================="
  exit 1
fi

echo "    ✓ Nenhuma vulnerabilidade HIGH/CRITICAL encontrada."

# ---------------------------------------------------
# 2. Verificações de ambiente
# ---------------------------------------------------

if ! docker system info > /dev/null 2>&1; then
  echo "Docker não está rodando."
  exit 1
fi

if ! docker login "${REGISTRY}" > /dev/null 2>&1; then
  echo "Você não está logado no ${REGISTRY}."
  echo ""
  echo "Faça login com:"
  if [[ "${REGISTRY}" == "ghcr.io" ]]; then
    echo "   echo \"\$GHCR_TOKEN\" | docker login ghcr.io -u SEU_USUARIO --password-stdin"
  else
    echo "   docker login ${REGISTRY}"
  fi
  exit 1
fi

BUILDER_NAME="local-multi"
if ! docker buildx inspect "${BUILDER_NAME}" > /dev/null 2>&1; then
  docker buildx create --name "${BUILDER_NAME}" --driver docker-container --driver-opt network=host --use
else
  docker buildx use "${BUILDER_NAME}"
fi
docker buildx inspect --bootstrap > /dev/null

# ---------------------------------------------------
# 3. Build e push das imagens
# ---------------------------------------------------

BUILD_CACHE_FLAG=""
if [ "${NO_CACHE}" = true ]; then
  BUILD_CACHE_FLAG="--no-cache"
  echo ""
  echo "🚫 Build SEM cache"
fi

echo ""
echo ">>> [1/2] Building gateway image (${PLATFORM})..."
echo "          Context: ./gateway"
echo "          Dockerfile: ./gateway/Dockerfile"

docker buildx build \
  --platform "${PLATFORM}" \
  --push \
  ${BUILD_CACHE_FLAG} \
  -f "./gateway/Dockerfile" \
  -t "${IMAGE_GATEWAY}:${TAG_BRANCH}" \
  -t "${IMAGE_GATEWAY}:${TAG_SHA}" \
  ./gateway

    echo "    ✓ Gateway image construída e enviada:"
    echo "      - ${IMAGE_GATEWAY}:${TAG_BRANCH}"
    echo "      - ${IMAGE_GATEWAY}:${TAG_SHA}"

echo ""
echo ">>> [2/2] Building frontend image (${PLATFORM})..."
echo "          Context: ./frontend"
echo "          Dockerfile: ./frontend/Dockerfile"
echo "          Build info: ${TAG_SHA}, ${BUILD_DATE} ${BUILD_TIME} (${BUILD_TZ})"

docker buildx build \
  --platform "${PLATFORM}" \
  --push \
  ${BUILD_CACHE_FLAG} \
  --build-arg IMAGE_TAG="${TAG_SHA}" \
  --build-arg BUILD_DATE="${BUILD_DATE}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  --build-arg COMMIT="${BUILD_COMMIT}" \
  --build-arg BUILD_TIMESTAMP="${BUILD_TIMESTAMP}" \
  -f "./frontend/Dockerfile" \
  -t "${IMAGE_FRONTEND}:${TAG_BRANCH}" \
  -t "${IMAGE_FRONTEND}:${TAG_SHA}" \
  ./frontend

    echo "    ✓ Frontend image construída e enviada:"
    echo "      - ${IMAGE_FRONTEND}:${TAG_BRANCH}"
    echo "      - ${IMAGE_FRONTEND}:${TAG_SHA}"

echo ""
echo "============================================="
echo "  Build and Push concluído!"
echo "============================================="
echo ""
echo "Imagens disponíveis no GHCR:"
echo ""
echo "  Gateway:"
echo "    ${IMAGE_GATEWAY}:${TAG_BRANCH}"
echo "    ${IMAGE_GATEWAY}:${TAG_SHA}"
echo ""
echo "  Frontend:"
echo "    ${IMAGE_FRONTEND}:${TAG_BRANCH}"
echo "    ${IMAGE_FRONTEND}:${TAG_SHA}"
echo ""
echo "============================================="
