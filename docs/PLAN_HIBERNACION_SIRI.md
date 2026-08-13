# Plan: hibernar la PC mediante Siri

## Objetivo

Permitir que el usuario diga «Oye Siri, hiberna mi computadora» y que un Atajo de iPhone solicite de forma segura la hibernación de una PC con Windows.

El diseño evita exponer la PC directamente a Internet y evita guardar secretos capaces de suplantar al servidor en ella.

## Arquitectura propuesta

```text
Siri → Atajo de iPhone → Cloudflare Proxy → router → Nginx → API firmadora en Docker → POST HTTPS al agente Go en Windows → hibernar
```

1. Siri ejecuta el Atajo «Hibernar computadora» en el iPhone.
2. El Atajo realiza una petición HTTPS al dominio protegido por Cloudflare.
3. Cloudflare reenvía la petición HTTPS al router doméstico, que publica únicamente el puerto 443 hacia Nginx.
4. Nginx aplica los controles de entrada y reenvía la solicitud al contenedor de la API firmadora.
5. La API autentica la solicitud, construye el comando y lo firma con su clave privada.
6. La API envía un `POST` HTTPS firmado al agente Go de la PC.
7. El agente verifica la firma, expiración y protección contra repeticiones.
8. Si todo es válido, registra el evento y ejecuta `shutdown /h`.

El agente de la PC recibe órdenes mediante un endpoint `POST` HTTPS. Nginx es el único componente publicado hacia Internet; el servicio Go no se expone directamente.

## Componentes a desarrollar en la PC

Crear un agente nativo en Go para Windows, compilado como ejecutable e instalado como servicio de Windows o tarea programada al iniciar el sistema.

Responsabilidades:

- Exponer un endpoint `POST` HTTPS únicamente para la API firmadora local.
- Almacenar únicamente la clave pública de la API firmadora.
- Aceptar una lista cerrada de acciones; inicialmente, solo `hibernate`.
- Verificar criptográficamente cada comando antes de ejecutarlo.
- Ejecutar `shutdown.exe /h`.
- Registrar comandos aceptados y rechazados sin incluir secretos.
- Reiniciarse automáticamente tras un fallo.

El agente no se ejecuta dentro de Docker: se distribuye como binario de Windows y se comunica mediante HTTPS con la API firmadora.

Estructura local sugerida:

```text
C:\ProgramData\RemotePcController\
  config.json
  signer-public-key.pem
  logs\
```

La clave privada nunca se copia ni se genera en la PC.

### Configuración inicial del agente Windows

Al instalar el agente como servicio, el instalador debe solicitar al menos:

- `PORT`: puerto en el que escuchará el endpoint HTTPS del agente.
- `PUBLIC_KEY`: clave pública de la API firmadora usada para verificar las órdenes.

El instalador debe escribir estos valores en un archivo `.env` situado junto al ejecutable. Por ejemplo:

```text
C:\Program Files\RemotePcController\
  remote-pc-controller.exe
  .env
```

Ejemplo de contenido:

```dotenv
PORT=9876
PUBLIC_KEY_BASE64=LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0tLi4u
```

La clave pública PEM debe guardarse codificada en Base64 de una sola línea (`PUBLIC_KEY_BASE64`) o mediante una ruta explícita (`PUBLIC_KEY_PATH`). No conviene almacenar un PEM multilínea directamente en un archivo `.env`.

Al arrancar, el binario busca por defecto `.env` en el directorio que contiene el propio ejecutable. No debe usar el directorio de trabajo actual, porque un servicio iniciado por SCM puede tener un directorio de trabajo diferente. Si no existe el archivo, o si el puerto o la clave no son válidos, el agente debe rechazar el arranque y dejar un error claro en sus logs.

El instalador debe restringir la modificación de `.env` a administradores y a la cuenta que ejecute el servicio. Aunque la clave pública no es secreta, impedir cambios no autorizados evita sustituirla por una clave de un atacante.

## Servicio intermedio dockerizado

La API pública y firmadora se ejecuta en un contenedor Docker en el servidor intermedio. Nginx permanece delante de ella como reverse proxy y es el único componente que recibe tráfico de Internet.

Responsabilidades del contenedor:

- Recibir la solicitud autenticada enviada por el Atajo de Siri a través de Nginx.
- Validar autorización, dispositivo solicitado y acción permitida.
- Generar `commandId`, nonce y periodo de validez.
- Firmar el comando con la clave privada.
- Enviar el `POST` HTTPS firmado al endpoint del agente Windows.
- Registrar resultados operativos sin registrar secretos ni claves.

Reglas de despliegue:

- No incluir la clave privada, tokens ni certificados en la imagen Docker.
- Inyectar secretos mediante un gestor de secretos o archivos montados con permisos de solo lectura.
- Publicar el puerto del contenedor solo hacia Nginx; no exponerlo directamente en el router.
- Ejecutar el contenedor como usuario no root y con sistema de archivos de solo lectura cuando sea compatible.
- Definir política de reinicio, límites de recursos y logs persistentes.
- Mantener una imagen mínima y actualizar sus dependencias periódicamente.

## Protocolo de comandos

No se debe firmar un JSON producido libremente por cada implementación: pequeñas diferencias de orden o formato pueden cambiar los bytes firmados. En su lugar, se firma una representación canónica, por ejemplo:

```text
v1|command-id|device-id|hibernate|issued-at|expires-at|nonce
```

Datos mínimos equivalentes:

```json
{
  "version": 1,
  "commandId": "uuid",
  "deviceId": "mi-pc",
  "action": "hibernate",
  "issuedAt": 1780000000,
  "expiresAt": 1780000030,
  "nonce": "valor-aleatorio-largo"
}
```

La firma se transmite como Base64 en un campo o encabezado independiente. La respuesta a la API firmadora debe confirmar si el comando fue aceptado o rechazado; la hibernación se programa justo después de enviar esa confirmación.

## Seguridad

- Usar RSA-PSS con SHA-256 y una clave RSA de 3072 bits. Ed25519 también es una alternativa moderna y más simple.
- Mantener la clave privada exclusivamente en la API firmadora; la PC solo conserva su clave pública.
- Exigir TLS incluso cuando el comando esté firmado.
- Rechazar comandos vencidos; una vigencia de 30 a 60 segundos es suficiente.
- Validar que `deviceId` corresponda a la PC local.
- Generar `nonce` y `commandId` con un generador criptográficamente seguro.
- Persistir durante la ventana de vigencia los `nonce` o `commandId` ya procesados. El timestamp por sí solo no bloquea la repetición de una solicitud dentro de su periodo válido.
- Aplicar límite de frecuencia a los comandos.
- Proteger `config.json` y la clave pública con permisos adecuados del sistema.
- Ejecutar el agente con el menor privilegio posible.
- Registrar fecha, resultado, identificador de comando y motivo de rechazo, pero no cargas completas, firmas ni credenciales.

## Autenticación del iPhone a la API

El Atajo de Siri necesita autenticarse ante la API expuesta a través de Cloudflare y Nginx. No debe existir un token hardcodeado en el código de la PC.

Opciones:

1. Para uso personal: un secreto aleatorio largo guardado en el Atajo, rotatable y con permiso exclusivo para crear el comando `hibernate`.
2. Para mayor control: un proveedor de identidad o capa de acceso, por ejemplo Cloudflare Access, antes de permitir la creación del comando.

La autenticación del Atajo protege el acceso a la API; la firma asimétrica protege la orden que finalmente recibe la PC.

## Exposición de red y Nginx

- Cloudflare es el proxy público del dominio.
- El router reenvía únicamente el puerto TCP 443 a Nginx.
- Nginx termina o revalida TLS y se comunica por HTTPS con el contenedor de la API cuando sea posible.
- Nginx acepta únicamente las rutas y métodos necesarios, en particular `POST` para la solicitud de hibernación.
- Nginx aplica límites de tamaño de cuerpo y de frecuencia de peticiones.
- El firewall debe impedir el acceso público directo al agente Go. Si Nginx y el agente están en la misma PC, el agente debe escuchar en `127.0.0.1`; si están en equipos distintos, debe escuchar únicamente en la IP LAN permitida y restringirse mediante firewall.
- Se debe limitar el acceso al origen a los rangos de IP de Cloudflare para impedir que se eluda el proxy conectándose a la IP pública del hogar.

## Correcciones al borrador inicial

- Para extraer la clave pública de una clave privada RSA, usar:

  ```bash
  openssl rsa -in private_key.pem -pubout -out public_key.pem
  ```

  La opción `-pubin` se usa cuando la entrada ya es una clave pública.

- Usar `rsa.VerifyPSS` en vez de `rsa.VerifyPKCS1v15`.
- No ignorar errores de serialización, firma, lectura de claves ni ejecución de comandos.
- No confiar solo en el timestamp: implementar almacenamiento temporal de nonces o identificadores de comando.
- Cargar la clave pública desde una ruta fija y protegida, no desde el directorio de trabajo actual.

## Fases de implementación

### 1. Validación local

- Confirmar que la hibernación está activada con `powercfg /hibernate on`.
- Construir y compilar el agente Go mínimo como ejecutable de Windows.
- Verificar la ejecución controlada de `shutdown /h`.
- Añadir logs y política de reinicio.

### 2. Protocolo firmado

- Generar el par de claves.
- Implementar firma en la API firmadora y verificación RSA-PSS en el agente.
- Validar expiración, dispositivo, `commandId` y nonce.
- Crear pruebas para comandos válidos, firmas inválidas, expiraciones y reintentos.

### 3. API, Nginx y comunicación HTTP

- Crear el endpoint HTTPS que recibe la solicitud del Atajo a través de Cloudflare y Nginx.
- Dockerizar la API firmadora y gestionar sus secretos fuera de la imagen.
- Implementar su llamada `POST` HTTPS al agente de la PC.
- Configurar Nginx, firewall y el reenvío del puerto 443 en el router.
- Añadir registros de solicitudes, firmas, entregas y errores sin exponer datos sensibles.

### 4. Atajo de Siri

- Crear el Atajo «Hibernar computadora».
- Añadir una confirmación opcional antes de enviar el comando.
- Configurar la llamada HTTPS al dominio de la API.
- Probar tanto en Wi-Fi local como desde red móvil.

### 5. Operación y mantenimiento

- Instalar el agente como servicio.
- Definir rotación y revocación de claves.
- Documentar recuperación tras pérdida del iPhone, cambio de PC o sospecha de compromiso.
- Evaluar futuras acciones únicamente después de mantener una lista explícita de acciones permitidas.

### 6. Opcional: servicio nativo de Windows mediante SCM

Convertir el agente compilado en Go en un servicio nativo administrado por el Service Control Manager (SCM) de Windows. Esta es la vía oficial de Windows para que el proceso aparezca en `services.msc`, arranque automáticamente y pueda reiniciarse tras un fallo.

- Implementar el ciclo de vida del servicio con `golang.org/x/sys/windows/svc`: inicio, estado `running`, recepción de `Stop` y `Shutdown`, cierre limpio y estado `stopped`.
- Conservar el modo consola para desarrollo: al ejecutar `remote-pc-controller.exe` desde PowerShell, mostrar logs y detener mediante `Ctrl+C`; cuando SCM lo inicie, ejecutar el modo servicio.
- Instalar el binario en una ruta estable, por ejemplo `C:\Program Files\RemotePcController\remote-pc-controller.exe`, y guardar configuración y logs en `C:\ProgramData\RemotePcController\`.
- Durante la instalación, solicitar el puerto y la clave pública de la API firmadora, validarlos y crear `.env` junto al ejecutable. Al actualizar, conservar el archivo de configuración salvo que el usuario el cambie expresamente.
- Registrar el servicio durante desarrollo con `sc.exe`; para distribución, incluir el registro, inicio, actualización y desinstalación en un instalador gráfico MSI o EXE.
- Configurar inicio automático retrasado y recuperación ante fallos (reiniciar el servicio tras fallos controlados).
- Ejecutar el servicio con una cuenta de Windows de privilegios mínimos y comprobar que puede ejecutar la acción de hibernación autorizada.
- Verificar desde `services.msc` que el servicio aparece, inicia con Windows, se detiene correctamente y se recupera tras un cierre inesperado.

## Criterios de aceptación

- La PC no tiene puertos entrantes expuestos a Internet.
- La PC no contiene una credencial capaz de emitir comandos válidos.
- Un comando alterado, vencido, dirigido a otra PC o repetido es rechazado.
- Un comando válido hiberna la PC y deja un registro auditable.
- Siri puede iniciar el flujo desde el Atajo de iPhone.
