# Despliegue de Relay (Docker)

Esta carpeta contiene la configuración necesaria para ejecutar la API firmadora **Relay** en un contenedor Docker con privilegios mínimos, preparado para integrarse con un servidor **Nginx independiente** ya existente.

---

## Arquitectura de Flujo

```text
[Atajo Siri / iPhone]
        │ HTTPS (con certificado SSL gestionado en Cloudflare / NPM)
        ▼
[Cloudflare Proxy]
        │ HTTPS (Puerto 443)
        ▼
[Nginx Proxy Manager / Nginx Soberano] (Termina TLS aquí)
        │ HTTP plano (Proxy interno hacia http://127.0.0.1:8080 sin certificados)
        ▼
[Contenedor Relay (Docker)] ──(Firma RSA-PSS)──► [Sentinel (PC Windows LAN: 9876)]
```

---

## Estructura de Archivos

```text
relay/
├── Dockerfile                  # Construcción multi-stage (Go Alpine → Alpine mínimo)
├── .dockerignore               # Filtro de archivos para el contexto de Docker
└── deploy/
    ├── docker-compose.yml      # Definición del contenedor Relay y sus restricciones
    ├── .env.example            # Plantilla de variables de entorno
    └── README.md               # Esta guía
```

---

## Guía de Puesta en Marcha

### 1. Generar el Par de Claves Criptográficas RSA (3072 bits)

Ejecuta en tu terminal para generar la clave privada (para el `.env` de Relay) y extraer la clave pública (para el `.env` de Sentinel):

```bash
# 1. Crear la clave privada RSA de 3072 bits
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out private_key.pem

# 2. Extraer la clave pública correspondiente
openssl rsa -in private_key.pem -pubout -out public_key.pem

# 3. Mostrar la clave privada para copiarla en el .env de Relay:
cat private_key.pem

# 4. Mostrar la clave pública para copiarla en el .env de Sentinel:
cat public_key.pem
```

> **Importante**:
> - El contenido de `private_key.pem` se copia y pega como texto multilínea en la variable `PRIVATE_KEY` del archivo `deploy/.env` de Relay.
> - El contenido de `public_key.pem` se copia y pega como texto multilínea en la variable `PUBLIC_KEY` del archivo `.env` de Sentinel en la PC Windows.
> - Elimina los archivos temporales `.pem` una vez copiados al `.env`.

---

### 2. Generar el Secreto de API (`API_SECRET`)

Genera una cadena aleatoria segura para la autenticación entre el Atajo de Siri y Relay:

```bash
openssl rand -hex 32
```

---

### 3. Configurar el Archivo de Entorno (`.env`)

Copia la plantilla `.env.example` a `.env` dentro de `relay/deploy/`:

```bash
cp deploy/.env.example deploy/.env
```

Edita `deploy/.env` con tus valores reales:

```dotenv
HOST_PORT=8080
LISTEN_ADDR=:8080
API_SECRET=<tu-secreto-generado-en-el-paso-2>
DEVICE_ID=sentinel-office
SENTINEL_URL=http://<IP_LAN_PC_WINDOWS>:9876/v1/commands
VALIDITY_SECONDS=30
MAX_BODY_BYTES=16384

# Clave privada RSA multilínea pegada directamente:
PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0...
...
-----END RSA PRIVATE KEY-----"
```

---

### 4. Iniciar el Servicio con Docker Compose

Desde la carpeta `relay/deploy/`:

```bash
# Construir la imagen e iniciar en segundo plano
docker compose up -d --build

# Verificar el estado y healthcheck
docker compose ps

# Ver registros de ejecución
docker compose logs -f relay
```

---

## Requisitos de Alto Nivel para el Nginx Existente

Dado que tu servidor Nginx se administra de forma soberana e independiente, debes incorporar en su configuración las siguientes directivas y requisitos de seguridad para enrutar el tráfico hacia Relay:

1. **Terminación TLS y Exposición de Red**:
   - Nginx debe ser el único servicio expuesto a Internet (puerto 443) detrás del proxy de Cloudflare.
   - Usar certificados TLS válidos (ej. Cloudflare Origin Certificate o Let's Encrypt).

2. **Control Estricto de Métodos y Rutas**:
   - **`/health`**: Permitir exclusivamente peticiones con método `GET` para monitorización.
   - **`/v1/commands`**: Permitir exclusivamente peticiones con método `POST`.
   - **Cualquier otra ruta**: Debe ser rechazada con código `404 Not Found` o `403 Forbidden`.

3. **Límite de Tamaño de Cuerpo (`client_max_body_size`)**:
   - Limitar el cuerpo de la petición a `16k` (`client_max_body_size 16k;`) para mitigar ataques de denegación de servicio.

4. **Límite de Frecuencia (Rate Limiting)**:
   - Configurar una zona de limitación de tasa (ej. `limit_req_zone` basada en la IP real del cliente) con una tasa baja (ej. 1 a 2 peticiones/segundo y ráfaga pequeña) sobre `/v1/commands` para evitar repeticiones o ataques de fuerza bruta.

5. **Propagación de Cabeceras al Proxy**:
   - Mantener la cabecera `Authorization: Bearer <API_SECRET>` intacta hacia Relay.
   - Enviar cabeceras de identificación de cliente: `Host`, `X-Real-IP`, `X-Forwarded-For` y `X-Forwarded-Proto`.
   - Si se utiliza Cloudflare, restaurar la IP real mediante `CF-Connecting-IP` / `set_real_ip_from`.

6. **Destino del Proxy (Upstream)**:
   - Redirigir las solicitudes a `http://127.0.0.1:8080`.

---

## Verificación y Pruebas

### 1. Comprobar Healthcheck localmente

```bash
curl -i http://127.0.0.1:8080/health
```
*Respuesta esperada: `HTTP/1.1 200 OK` con contenido `ok`.*

### 2. Probar Solicitud de Comando a través de Relay

```bash
curl -i -X POST http://127.0.0.1:8080/v1/commands \
  -H "Authorization: Bearer <TU_API_SECRET>" \
  -H "Content-Type: application/json" \
  -d '{"action": "hibernate"}'
```

- Si Sentinel está en línea y recibe la orden firmada: `HTTP/1.1 202 Accepted` con `{"status":"accepted","commandId":"..."}`.
- Si la cabecera `Authorization` no coincide: `HTTP/1.1 401 Unauthorized`.
- Si el cuerpo o la acción son inválidos: `HTTP/1.1 400 Bad Request`.
- Si Sentinel no está accesible en la LAN: `HTTP/1.1 502 Bad Gateway`.
