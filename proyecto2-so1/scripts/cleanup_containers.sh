#!/bin/bash
# cleanup_containers.sh
# Detiene y elimina TODOS los contenedores del proyecto.

IDS=$(docker ps -aq --filter "label=proyecto2.managed=true")

if [ -z "$IDS" ]; then
    echo "No hay contenedores del proyecto para limpiar."
    exit 0
fi

echo ">> Deteniendo y eliminando contenedores del proyecto..."
docker rm -f $IDS

echo ">> Listo."
