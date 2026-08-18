# ADR 0001 — Generación de código en build-time en vez de reflection en runtime

## Estado
Aceptado

## Contexto
GoFast busca dar la misma experiencia de desarrollo que FastAPI:
un desarrollador escribe un struct con tags, y el framework
genera automáticamente validación y documentación OpenAPI,
sin que el desarrollador escriba ese código a mano.

Existe un framework Go previo con el mismo objetivo (Huma),
que resuelve esto usando reflection en tiempo de ejecución:
cada request que llega, el framework recorre el struct con
el paquete `reflect` para validar y serializar.

La cultura de Go tiende a desconfiar de la automatización
oculta (reflection en runtime, "magia") porque es difícil de
auditar y de debuggear. Al mismo tiempo, todo desarrollador
valora no repetir código de validación y documentación a mano.

## Decisión
GoFast genera el código de validación en tiempo de
compilación (build-time), usando `go generate` y el paquete
`reflect` una sola vez, sobre el CÓDIGO FUENTE, no sobre
requests en producción. El resultado es un archivo `.gen.go`
real, legible, versionado en el repositorio del usuario.

En runtime, GoFast no usa reflection para validar. Usa una
interfaz (`Validatable`) que el código generado implementa.
El costo de reflection se paga una sola vez, al desarrollar;
nunca se paga en producción, por request.

## Alternativas consideradas

- **Reflection en runtime (enfoque de Huma)**
  Descartada como decisión central. Es más simple de
  implementar, pero paga el costo de reflection en cada
  request, y el código que valida nunca es visible ni
  auditable por el desarrollador que usa el framework.

- **Sin automatización, todo manual (enfoque Gin/Echo actual)**
  Descartada. Es el problema que GoFast busca resolver, no
  una alternativa válida.

## Consecuencias

**Ganamos:**
- Cero overhead de reflection en producción, por diseño.
- Código de validación auditable: aparece en el `git diff`
  del desarrollador como Go normal, no como caja negra.
- Alineado con la cultura Go de desconfiar de la magia oculta,
  sin renunciar a la automatización.

**Sacrificamos:**
- Más complejidad de implementación que un enfoque de
  reflection en runtime (necesitamos un generador de código,
  no solo una función que recorra structs).
- Un paso extra en el flujo de desarrollo (`gofast generate`),
  que el desarrollador debe recordar correr — mitigado con
  verificación de hash en build (ver ADR futuro sobre esto).

**Riesgo aceptado conscientemente:**
- Si el generador de código resulta más difícil de mantener
  de lo previsto, este ADR podría revisarse. En ese caso, se
  crea un ADR nuevo que referencia a este, no se edita este
  archivo.