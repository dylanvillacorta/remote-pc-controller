# Integration tests

Las pruebas de integración deben levantar Relay y un Sentinel con un ejecutor falso, y comprobar:

1. comando válido;
2. firma alterada;
3. dispositivo incorrecto;
4. comando vencido;
5. repetición del mismo `commandId`;
6. entrega y respuesta de Relay.
