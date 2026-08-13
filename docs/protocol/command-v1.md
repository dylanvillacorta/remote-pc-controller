# Command Protocol v1

## Comando

Campos JSON obligatorios:

```json
{
  "version": 1,
  "commandId": "uuid",
  "deviceId": "sentinel-office",
  "action": "hibernate",
  "issuedAt": 1780000000,
  "expiresAt": 1780000030,
  "nonce": "random-value",
  "signature": "base64-rsa-pss"
}
```

Relay firma exactamente:

```text
v1|commandId|deviceId|action|issuedAt|expiresAt|nonce
```

Sentinel debe validar la firma RSA-PSS con SHA-256, el dispositivo, la acción permitida, la ventana temporal y la unicidad de `commandId`/`nonce`.
