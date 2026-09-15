#!/bin/sh
echo "[intruso] iniciando simulacion de actividad sospechosa..."

while true; do
    echo "[intruso] $(date): intentando leer archivos sensibles (dentro de mi propio contenedor)"

    for f in /etc/shadow /etc/passwd /root/.ssh/id_rsa; do
        if [ -r "$f" ]; then
            echo "[intruso] pude leer: $f"
        else
            echo "[intruso] sin permiso para leer: $f"
        fi
    done

    sleep 15
done