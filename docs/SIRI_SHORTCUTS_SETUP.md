# Guía de Configuración: Atajos de Siri (Apple Shortcuts)

Esta guía detalla la configuración del Atajo de Siri para interactuar de forma segura con el servidor **Relay** y ejecutar comandos de suspensión/hibernación en tu PC.

---

## 1. Estructura de la Petición HTTP desde Siri

Para que el atajo active la hibernación en la PC, Siri debe enviar una solicitud HTTP al endpoint de tu servidor Relay. La estructura técnica requerida es:

* **URL**: `https://<subdominio>.<tudominio>.com/v1/commands`
* **Método**: `POST`
* **Cabeceras (Headers)**:
  * `Authorization`: `Bearer <TU_API_SECRET>`
  * `Content-Type`: `application/json`
  * `Host`: `<subdominio>.<tudominio>.com` *(Requerido en HTTP/1.1; algunos clientes o proxies lo exigen explícitamente)*
* **Cuerpo (JSON Body)**:
  ```json
  {
      "deviceId": "sentinel-office",
      "action": "hibernate"
  }
  ```

---

## 2. Configuración en la App "Atajos" (Shortcuts) de iOS / macOS

Sigue estos pasos detallados para crear la acción en tu dispositivo Apple:

1. Abre la aplicación **Atajos** (Shortcuts).
2. Toca el botón **+** en la esquina superior derecha para crear un nuevo atajo.
3. Asigna un nombre al atajo (ej. *"Suspender Computadora"* o *"Apagar PC"*). **Este nombre será la frase de voz exacta que Siri reconocerá para activar el atajo.**
4. Agrega la acción **"Obtener contenido de URL"** (Get contents of URL).
5. Configura la acción con los siguientes campos:
   * **URL**: Introduce la dirección de tu endpoint de Relay (ej. `https://relay.tudominio.com/v1/commands`).
   * Toca **"Mostrar más"** para configurar los parámetros adicionales:
     * **Método**: Selecciona `POST`.
     * **Cabeceras**:
       * Agrega una nueva cabecera con la clave `Authorization` y el valor `Bearer <TU_API_SECRET>` (reemplaza `<TU_API_SECRET>` con tu secreto real generado por el script).
       * Agrega una cabecera con la clave `Content-Type` y el valor `application/json`.
       * Agrega una cabecera con la clave `Host` y tu subdominio como valor (ej. `relay.tudominio.com`). *Esto es obligatorio para evitar errores 400 Bad Request en proxies inversos y balanceadores.*
     * **Cuerpo de la Petición**: Selecciona `JSON`.
       * Agrega un campo tipo `Texto` con la clave `deviceId` y el valor de tu dispositivo (ej. `sentinel-office` o `sentinel-dylanpc`).
       * Agrega un campo tipo `Texto` con la clave `action` y el valor `hibernate`.
6. Guarda el atajo.

---

## 3. Sugerencias de Subdominio (3er Nivel)

Para exponer el servicio Relay en la web, debes apuntar un registro CNAME o A en tu proveedor DNS (Cloudflare, GoDaddy, etc.) hacia el servidor donde se ejecuta Dokploy/Relay. 

Aquí tienes opciones profesionales de nombres de **3er nivel (subdominios)** para tu dominio principal:

### Opción A: Orientados al Servicio (Recomendado)
* **`relay.<tudominio>.com`**: Directo, limpio y técnico. Explica perfectamente que actúa como puente/relay.
* **`gateway.<tudominio>.com`**: Formal y corporativo. Indica que es una pasarela de acceso a tu red local.

### Opción B: Orientados a la Acción o Dispositivo
* **`control.<tudominio>.com`**: Intuitivo. Ideal si planeas añadir comandos para más dispositivos en el futuro.
* **`trigger.<tudominio>.com`**: Corto y descriptivo. Sugiere que es el disparador o interruptor de una automatización.

### Opción C: Orientado al Asistente
* **`siri.<tudominio>.com`**: Muy específico. Deja claro que su único propósito es servir como puente para tus Atajos de Siri.
