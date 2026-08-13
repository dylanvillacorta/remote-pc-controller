# Sentinel

Agente Windows que valida órdenes firmadas y solo puede hibernar la PC.

```text
sentinel/
  src/
    main.go      punto de entrada del binario
    app/          arranque, ciclo de vida y servicio SCM
    command/      protocolo, política funcional y caso de uso
    config/       lectura y validación de .env
    httpapi/      adaptador HTTP
    replay/       persistencia anti-repetición
    windows/      adaptador shutdown.exe /h
  deploy/         instalación, servicio Windows y configuración de producción
  tests/          pruebas de integración y escenarios end-to-end
```

## Diseño aplicado

Sentinel usa un diseño idiomático de Go: **functional core / imperative shell**, con **Functional Options** para inyectar dependencias.

- `CommandPolicy.Validate` es el núcleo funcional: recibe comando, política y hora; no hace I/O ni ejecuta acciones.
- `CommandService` orquesta los puertos pequeños `ReplayProtector`, `ActionExecutor` y `AuditLogger`.
- `App`, `ReplayStore`, `WindowsHibernateExecutor` y el servicio SCM forman la capa imperativa.
- `NewCommandService` se compone con `WithReplayProtector`, `WithActionExecutor`, `WithClock` y `WithAuditLogger`.

El reloj y el ejecutor se sustituyen en las pruebas, así que las reglas de seguridad se prueban sin abrir puertos ni llamar a `shutdown.exe`.
