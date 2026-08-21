# Manual Técnico y Guía de Instalación
## Proyecto 1 — Desarrollo, Conexión y Gestión de Contenedores en Entornos Virtualizados
**Sistemas Operativos 1 — USAC**
**Carnet:** 202300559

---

## 1. Resumen de la arquitectura

El proyecto simula un entorno de microservicios distribuidos usando 3 máquinas virtuales sobre KVM, cada una con un runtime de contenedores distinto, corriendo una API en Go que se comunica con las otras dos vía REST/HTTP. Una de las VMs además aloja un registro privado de imágenes (Zot).

| VM | Hostname | IP | Runtime | Servicio |
|---|---|---|---|---|
| VM1 | `vm1-containerd` | `192.168.122.179` | Containerd + nerdctl | API1 (puerto 8080) |
| VM2 | `vm2-podman` | `192.168.122.241` | Podman | API2 (puerto 8081) |
| VM3 | `vm3-docker-zot` | `192.168.122.112` | Docker | API3 (puerto 8082) + Zot (puerto 5000) |

Todas las VMs están en la red NAT por defecto de libvirt (`virbr0`, rango `192.168.122.0/24`) y se comunican entre sí sin restricciones.

---

## 2. Requisitos previos del host

- Ubuntu/Debian con soporte de virtualización activo en BIOS (verificado con `egrep -c '(vmx|svm)' /proc/cpuinfo` > 0)
- Al menos 8 GB de RAM libres (2 GB por VM x 3, más margen)
- ~60 GB de espacio en disco libre

---

## 3. Instalación de KVM y virt-manager (host)

```bash
sudo apt update
sudo apt install -y qemu-system-x86 libvirt-daemon-system libvirt-clients bridge-utils virt-manager
```

Verificar el servicio:
```bash
sudo systemctl status libvirtd
sudo virsh list --all
sudo virsh net-list --all   
```


---
## 4. Creación de las 3 máquinas virtuales

Se usó **Ubuntu Server 26.04** para minimizar el consumo de recursos
Para cada VM:

1. `virt-manager` → Crear nueva máquina virtual → Medio de instalación local (ISO)
2. Seleccionar la ISO de Ubuntu Server descargada
3. Sistema operativo: Ubuntu 24.04 LTS (compatibilidad más cercana disponible en el asistente)
4. Recursos: **2048 MB RAM**, **2 vCPU**, **15-20 GB de disco**
5. Nombre: `VM1-containerd`, `VM2-Podman`, `VM3-docker-zot`
6. Red: Red virtual `default` (NAT)
7. Durante la instalación:
   - Storage: layout automático (LVM), confirmar acción destructiva
   - **SSH configuration: marcar `[X] Instalar servidor OpenSSH`** (imprescindible para administración remota)
   - Featured Server Snaps: no seleccionar ninguno
   - Reboot Now al finalizar

### Verificación post-instalación (en cada VM)

```bash
hostname
ip a
```

### Prueba de conectividad SSH desde el host

```bash
ssh alberto@192.168.122.179   # VM1
ssh alberto@192.168.122.241   # VM2
ssh alberto@192.168.122.112   # VM3
```

### Prueba de red entre VMs

```bash
ping -c 3 192.168.122.179
ping -c 3 192.168.122.241
```

### Actualización del sistema (en cada VM)

```bash
sudo apt update && sudo apt upgrade -y
```

### Encender/apagar VMs tras un reinicio del host

```bash
sudo virsh list --all
sudo virsh start <nombre-vm>
```

---

## 5. Instalación de los runtimes de contenedores

### 5.1 VM1 — Containerd + nerdctl

```bash
sudo apt update
sudo apt install -y containerd
sudo systemctl enable --now containerd
```

Instalación de `nerdctl` (cliente CLI similar a Docker):
```bash
cd /tmp
NERDCTL_VERSION="2.0.2"
wget https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-amd64.tar.gz
sudo tar -xzf nerdctl-${NERDCTL_VERSION}-linux-amd64.tar.gz -C /usr/local/bin
```

Instalación de los plugins CNI (necesarios para que containerd cree redes para los contenedores):
```bash
cd /tmp
CNI_VERSION="1.6.2"
sudo mkdir -p /opt/cni/bin
wget https://github.com/containernetworking/plugins/releases/download/v${CNI_VERSION}/cni-plugins-linux-amd64-v${CNI_VERSION}.tgz
sudo tar -xzf cni-plugins-linux-amd64-v${CNI_VERSION}.tgz -C /opt/cni/bin
```

Instalación de BuildKit (necesario para `nerdctl build`, ya que containerd no incluye un builder integrado):
```bash
cd /tmp
BUILDKIT_VERSION="0.19.0"
wget https://github.com/moby/buildkit/releases/download/v${BUILDKIT_VERSION}/buildkit-v${BUILDKIT_VERSION}.linux-amd64.tar.gz
sudo tar -xzf buildkit-v${BUILDKIT_VERSION}.linux-amd64.tar.gz -C /usr/local
sudo /usr/local/bin/buildkitd > /tmp/buildkitd.log 2>&1 &
```

Prueba:
```bash
sudo nerdctl run --rm hello-world
```

### 5.2 VM2 — Podman

```bash
sudo apt update
sudo apt install -y podman
```

Prueba:
```bash
podman run --rm hello-world
```

> Podman es "daemonless" (no requiere un servicio corriendo en segundo plano) y trae su propio builder integrado, por lo que no necesitó configuración adicional.

### 5.3 VM3 — Docker

```bash
sudo apt install -y ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Prueba:
```bash
sudo docker run --rm hello-world
```

---

## 6. Código fuente de las APIs (Go)

Las 3 APIs comparten el mismo binario (`main.go`), configurado en tiempo de ejecución mediante variables de entorno (`API_ID`, `VM_ID`, `PORT`, y las URLs/VM de sus dos pares). Esto evita duplicar lógica.

### 6.1 Estructura del proyecto

```
proyecto1-so1/
├── api/            # código fuente base
├── api1/           # copia + Dockerfile → VM1
├── api2/           # copia + Dockerfile → VM2
└── api3/           # copia + Dockerfile → VM3
```

### 6.2 `main.go`

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const carnet = "202300559"

var (
	apiID   string
	vmID    string
	port    string
	peerURL = map[string]string{}
	peerVM  = map[string]string{}
)

type healthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type callResponse struct {
	APIName    string `json:"apiname"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "UP",
		Message:   fmt.Sprintf("API%s is Ready", apiID),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        vmID,
		Carnet:    carnet,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func makeCallHandler(targetID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := peerURL[targetID]
		targetVM := peerVM[targetID]

		connection := false
		message := fmt.Sprintf("ERROR: The API%s located on the %s is not working", targetID, targetVM)

		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			defer resp.Body.Close()
			var body healthResponse
			if json.NewDecoder(resp.Body).Decode(&body) == nil && body.Status == "UP" {
				connection = true
				message = fmt.Sprintf("The API%s located on the %s is working", targetID, targetVM)
			}
		}

		out := callResponse{
			APIName:    "API" + targetID,
			Message:    message,
			Connection: connection,
			Carnet:     carnet,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func main() {
	apiID = os.Getenv("API_ID")
	vmID = os.Getenv("VM_ID")
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	switch apiID {
	case "1":
		peerURL["2"] = os.Getenv("API2_URL")
		peerVM["2"] = os.Getenv("API2_VM")
		peerURL["3"] = os.Getenv("API3_URL")
		peerVM["3"] = os.Getenv("API3_VM")
	case "2":
		peerURL["1"] = os.Getenv("API1_URL")
		peerVM["1"] = os.Getenv("API1_VM")
		peerURL["3"] = os.Getenv("API3_URL")
		peerVM["3"] = os.Getenv("API3_VM")
	case "3":
		peerURL["1"] = os.Getenv("API1_URL")
		peerVM["1"] = os.Getenv("API1_VM")
		peerURL["2"] = os.Getenv("API2_URL")
		peerVM["2"] = os.Getenv("API2_VM")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	for targetID := range peerURL {
		path := fmt.Sprintf("GET /api%s/%s/call-api%s", apiID, carnet, targetID)
		mux.HandleFunc(path, makeCallHandler(targetID))
	}

	log.Printf("API%s escuchando en el puerto %s (VM: %s)", apiID, port, vmID)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
```

### 6.3 `go.mod`

```
module api

go 1.26
```

---

## 7. Dockerfile (multi-stage build)

El mismo Dockerfile se usó en las 3 carpetas (`api1/`, `api2/`, `api3/`). La primera etapa compila el binario con la imagen oficial de Go; la segunda copia solo el binario resultante a una imagen Alpine mínima, reduciendo el tamaño final a ~9-20 MB.

```dockerfile
# Etapa 1: compilar el binario
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o api main.go

# Etapa 2: imagen final, mínima
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api .
EXPOSE 8080
ENTRYPOINT ["./api"]
```

---

## 8. Construcción y despliegue de cada API

### 8.1 Copiar el código fuente a cada VM

Desde el host:
```bash
scp -r api1 alberto@192.168.122.179:~/
scp -r api2 alberto@192.168.122.241:~/
scp -r api3 alberto@192.168.122.112:~/
```

### 8.2 VM1 — build con nerdctl

> **Nota:** los nombres de imagen deben estar en minúsculas por requisito del estándar OCI/Docker (`API1-202300559` no es válido; se usó `api1-202300559`).

```bash
cd ~/api1
sudo nerdctl build -t api1-202300559:v1 .
sudo nerdctl images
```

Ejecución:
```bash
sudo nerdctl run -d --name api1-test -p 8080:8080 \
  -e API_ID=1 -e VM_ID=VM1 -e PORT=8080 \
  -e API2_URL=http://192.168.122.241:8081 -e API2_VM=VM2 \
  -e API3_URL=http://192.168.122.112:8082 -e API3_VM=VM3 \
  api1-202300559:v1
```

### 8.3 VM2 — build con Podman

```bash
cd ~/api2
podman build -t api2-202300559:v1 .
podman images
```

Ejecución:
```bash
podman run -d --name api2-test -p 8081:8081 \
  -e API_ID=2 -e VM_ID=VM2 -e PORT=8081 \
  -e API1_URL=http://192.168.122.179:8080 -e API1_VM=VM1 \
  -e API3_URL=http://192.168.122.112:8082 -e API3_VM=VM3 \
  api2-202300559:v1
```

### 8.4 VM3 — build con Docker

```bash
cd ~/api3
sudo docker build -t api3-202300559:v1 .
sudo docker images
```

Ejecución:
```bash
sudo docker run -d --name api3-test -p 8082:8082 \
  -e API_ID=3 -e VM_ID=VM3 -e PORT=8082 \
  -e API1_URL=http://192.168.122.179:8080 -e API1_VM=VM1 \
  -e API2_URL=http://192.168.122.241:8081 -e API2_VM=VM2 \
  api3-202300559:v1
```

---

## 9. Registro privado de imágenes (Zot) — VM3

### 9.1 Configuración

```bash
mkdir -p ~/zot/data
nano ~/zot/config.json
```

Contenido de `config.json`:
```json
{
  "distSpecVersion": "1.1.0",
  "storage": {
    "rootDirectory": "/var/lib/registry"
  },
  "http": {
    "address": "0.0.0.0",
    "port": "5000"
  },
  "log": {
    "level": "info"
  }
}
```

### 9.2 Levantar Zot como contenedor Docker

```bash
sudo docker run -d \
  --name zot \
  -p 5000:5000 \
  -v ~/zot/data:/var/lib/registry \
  -v ~/zot/config.json:/etc/zot/config.json \
  ghcr.io/project-zot/zot:latest
```

Verificación:
```bash
sudo docker ps
curl -v http://localhost:5000/v2/
# Respuesta esperada: HTTP/1.1 200 OK, header "Docker-Distribution-Api-Version: registry/2.0"
```

### 9.3 Configurar cada VM para permitir el registro sin TLS (HTTP)

Zot corre sin HTTPS en este entorno de laboratorio, por lo que cada runtime debe configurarse explícitamente para aceptar un registro "inseguro" en `192.168.122.112:5000`.

**VM1 (containerd/nerdctl):**
```bash
sudo mkdir -p /etc/containerd/certs.d/192.168.122.112:5000
sudo nano /etc/containerd/certs.d/192.168.122.112:5000/hosts.toml
```
```toml
server = "http://192.168.122.112:5000"

[host."http://192.168.122.112:5000"]
  capabilities = ["pull", "resolve", "push"]
  skip_verify = true
```

### 9.4 Publicar (push) las imágenes al registro

**Desde VM1:**
```bash
sudo nerdctl image convert --oci --platform linux/amd64 api1-202300559:v1 api1-202300559:oci
sudo nerdctl tag api1-202300559:oci 192.168.122.112:5000/api1-202300559:v1
sudo nerdctl push 192.168.122.112:5000/api1-202300559:v1
```

> **Nota técnica:** containerd/nerdctl genera el manifest en formato Docker v2 por defecto, que esta versión de Zot no acepta (`415 Unsupported Media Type`). La solución es convertir la imagen al formato estándar OCI antes de subirla.

**Desde VM2 (Podman):**
```bash
podman push --tls-verify=false api2-202300559:v1 192.168.122.112:5000/api2-202300559:v1
```

**Desde VM3 (Docker):**
```bash
sudo docker tag api3-202300559:v1 192.168.122.112:5000/api3-202300559:v1
sudo docker push 192.168.122.112:5000/api3-202300559:v1
```

> Docker por defecto también rechaza registros sin TLS. Es necesario agregar `"insecure-registries": ["192.168.122.112:5000"]` en `/etc/docker/daemon.json` y reiniciar el servicio (`sudo systemctl restart docker`).

---

## 10. Pruebas de validación funcional

### 10.1 Endpoint `/health` de cada API

```bash
curl http://192.168.122.179:8080/health
curl http://192.168.122.241:8081/health
curl http://192.168.122.112:8082/health
```

Formato de respuesta esperado:
```json
{"status":"UP","message":"API1 is Ready","timestamp":"2026-08-20T06:18:09Z","VM":"VM1","carnet":"202300559"}
```

### 10.2 Comunicación cruzada entre las 3 APIs (a través de la red real)

```bash
curl http://192.168.122.179:8080/api1/202300559/call-api2
curl http://192.168.122.179:8080/api1/202300559/call-api3
curl http://192.168.122.241:8081/api2/202300559/call-api1
curl http://192.168.122.241:8081/api2/202300559/call-api3
curl http://192.168.122.112:8082/api3/202300559/call-api1
curl http://192.168.122.112:8082/api3/202300559/call-api2
```

Resultado obtenido (todas las combinaciones con `connection:true`):
```json
{"apiname":"API2","message":"The API2 located on the VM2 is working","connection":true,"carnet":"202300559"}
{"apiname":"API3","message":"The API3 located on the VM3 is working","connection":true,"carnet":"202300559"}
{"apiname":"API1","message":"The API1 located on the VM1 is working","connection":true,"carnet":"202300559"}
{"apiname":"API3","message":"The API3 located on the VM3 is working","connection":true,"carnet":"202300559"}
{"apiname":"API1","message":"The API1 located on the VM1 is working","connection":true,"carnet":"202300559"}
{"apiname":"API2","message":"The API2 located on the VM2 is working","connection":true,"carnet":"202300559"}
```

### 10.3 Prueba del manejo de errores

Con API3 detenida (`Ctrl+C` o `docker stop api3-test`):
```bash
curl http://192.168.122.179:8080/api1/202300559/call-api3
```

Resultado esperado:
```json
{"apiname":"API3","message":"ERROR: The API3 located on the VM3 is not working","connection":false,"carnet":"202300559"}
```

---

## 11. Problemas encontrados y soluciones (bitácora técnica)

| Problema | Causa | Solución |
|---|---|---|
| `virt-manager`: "No se pudo conectar a libvirt qemu:///system" | Usuario sin permisos en el grupo `libvirt` (decisión deliberada de no usar `usermod`) | Ejecutar `sudo virt-manager` |
| `libvirtd` se caía justo después de arrancar | Falló la creación del usuario de sistema `libvirt-qemu` durante la instalación del paquete | `sudo apt install --reinstall libvirt-daemon-system` |
| `qemu-kvm` no se pudo instalar con `apt` | El paquete cambió de nombre en versiones recientes de Ubuntu | Instalar `qemu-system-x86` en su lugar |
| `nerdctl build` falla: falta `buildctl`/`buildkitd` | containerd no incluye un builder integrado como Docker | Instalar BuildKit por separado y arrancar `buildkitd` |
| `nerdctl run`: falta binario `bridge` de CNI | El paquete `containerd` de apt no incluye los plugins CNI | Descargar e instalar los CNI plugins en `/opt/cni/bin` |
| `invalid reference format: ... must be lowercase` | Docker/OCI no permite mayúsculas en nombres de imagen | Usar minúsculas: `api1-202300559` en vez de `API1-202300559` |
| Error de sintaxis Go al compilar en VM2 | El bloque `var (...)` quedó sin cerrar correctamente al copiar el archivo | Corregir el archivo agregando el `)` de cierre |
| `push` a Zot falla con `415 Unsupported Media Type` | Formato de manifest Docker v2 no aceptado por esta versión de Zot | Convertir la imagen a formato OCI antes del push (`nerdctl image convert --oci`) |

---

## 12. Personalización aplicada

- Carnet utilizado en todas las rutas y respuestas: **202300559**
- Nombres de imágenes: `api1-202300559`, `api2-202300559`, `api3-202300559` (en minúsculas por requisito técnico de la especificación OCI)
- Tags: `v1` (a criterio del estudiante, según lo permite el enunciado)

---

## 13. Repositorio

- Nombre: `202300559_LAB_SO1_2S2026`
- Colaboradores agregados: `@JoseLorenzana272`, `@KINGR0X`