DOCKER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../docker/images" && pwd)"

echo ">> Construyendo imagen: proyecto2-low-consumo"
docker build -t proyecto2-low-consumo "$DOCKER_DIR/low-consumo"

echo ">> Construyendo imagen: proyecto2-alto-ram"
docker build -t proyecto2-alto-ram "$DOCKER_DIR/alto-ram"

echo ">> Construyendo imagen: proyecto2-alto-cpu"
docker build -t proyecto2-alto-cpu "$DOCKER_DIR/alto-cpu"

echo ">> Construyendo imagen: proyecto2-intruso"
docker build -t proyecto2-intruso "$DOCKER_DIR/intruso"

echo ">> Listo. Imagenes construidas:"
docker images | grep proyecto2