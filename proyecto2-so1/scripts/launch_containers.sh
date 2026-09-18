#!/bin/bash
#!/bin/bash
set -u  

NUM_CONTAINERS=5

POOL=(
    "low-consumo"
    "low-consumo"
    "alto-ram"
    "alto-ram"
    "alto-cpu"
    "alto-cpu"
    "intruso"
)
image_for_type() {
    case "$1" in
        low-consumo) echo "proyecto2-low-consumo" ;;
        alto-ram)    echo "proyecto2-alto-ram" ;;
        alto-cpu)    echo "proyecto2-alto-cpu" ;;
        intruso)     echo "proyecto2-intruso" ;;
    esac
}

TIMESTAMP=$(date +%s)

echo "=== $(date): lanzando $NUM_CONTAINERS contenedores ==="

for i in $(seq 1 "$NUM_CONTAINERS"); do
    idx=$((RANDOM % ${#POOL[@]}))
    tipo="${POOL[$idx]}"
    imagen=$(image_for_type "$tipo")

    nombre="p2_${tipo}_${TIMESTAMP}_${i}"

    docker run -d \
        --name "$nombre" \
        --label "proyecto2.tipo=${tipo}" \
        --label "proyecto2.managed=true" \
        --security-opt apparmor=unconfined \
        "$imagen" \
        > /dev/null

    if [ $? -eq 0 ]; then
        echo "  [OK] $nombre (tipo=$tipo, imagen=$imagen)"
    else
        echo "  [ERROR] no se pudo lanzar $nombre (tipo=$tipo)"
    fi
done

echo "=== Contenedores del proyecto actualmente corriendo ==="
docker ps --filter "label=proyecto2.managed=true" \
    --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"