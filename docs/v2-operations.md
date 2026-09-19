# Operar v2

El servicio completo sigue en un único proceso y contenedor, puerto **8080**. `docker compose up -d --build` inicia API, worker y mantenimiento; el playground está en `http://localhost:8080/playground`. Seleccionar **v2 · Fragmentos** para publicación progresiva. V1 sigue disponible y seleccionado inicialmente.

## Recursos y límites

V2 guarda resultados y checkpoints en el mismo SQLite persistente. **1 GiB es capacidad máxima de resultado por job, no RAM reservada**. `executions.v2` configura:

| Campo | Default | Aplicación |
|---|---:|---|
| `maxPublishedBytes` | 1,073,741,824 | Bytes JSON de fragmentos por job, reservando 256 KiB para cierre. |
| `maxStoredBytes` | 8,589,934,592 | Bytes lógicos globales de chunks, proyección, inputs y checkpoints; reserva cierre por job activo. |
| `maxChunks` | 1,000,000 | Fragmentos por job, con un lugar reservado para terminal. |
| `maxNodes` | 1,000,000 | Nodos JSON publicados por job. |
| `maxPendingTasks` | 100,000 | Continuaciones persistidas pendientes por job. |

El runner inicial usa **un recorrido secuencial por ejecución** con una pila durable acotada. `executions.maxConcurrent` limita jobs simultáneos (default cuatro); cuotas globales siguen aplicando. No crea una goroutine por nodo. La unidad de checkpoint es el coordinador y su pila, por lo que no necesita encolar todas las futuras ramas. Esto mantiene la memoria independiente del resultado acumulado y puede ofrecer menos paralelismo dentro de un job que v1.

Cada respuesta upstream sigue limitada por `fetch.maxResponseBytes` (default 4 MiB), profundidad y cantidad de valores. El heap depende de páginas decodificadas, ancestros necesarios, concurrencia, cache y fetch; JSON en memoria puede ocupar más que sus bytes en red. Los lotes de publicación se cierran al alcanzar 128 pasos/fragmentos o aproximadamente 1 MiB, además de cada respuesta aceptada. Una llamada lenta puede retrasar las otras ramas del mismo job.

SQLite ocupa más espacio físico que el JSON: cifrado serializado, índices, proyección para consultas, páginas libres y WAL. `maxStoredBytes` es una cota **lógica**, no del tamaño de archivos. `storage.minimumFreeDiskBytes` sigue comprobándose antes de avanzar; dejar espacio para escrituras, cierre y backups. Borrar filas permite reutilizar páginas, pero no encoge inmediatamente el archivo. Los inputs aceptados se eliminan cuando ninguna continuación los necesita. Los resultados/proyección expiran por `storage.resultRetention`; la metadata dura `metadataRetention`.

Compose conserva el límite de contenedor de 1 GiB y `GOMEMLIMIT=768MiB`; son límites operativos generales del proceso, independientes del tamaño permitido para cada resultado.

## Evidencia reproducible

```sh
make check
make playground-test
make test-v2-large
```

Aceptación ejecutada el 2026-09-18, Go local y SQLite cifrado temporal, un job con 1,024 páginas, ocho escalares de aproximadamente 128 KiB por página:

| Medida | Resultado |
|---|---:|
| Bytes publicados | 1,071,702,500 (~1,022 MiB) |
| Pico de HeapAlloc muestreado cada 10 ms | 21,184,184 bytes (~20.2 MiB) |
| Archivos SQLite, incluyendo WAL | 2,891,773,040 bytes (~2.69 GiB) |
| Tiempo del fixture | 23.516 s |
| Reapertura/recuperación intermedia | 1 |
| Páginas confirmadas solicitadas de nuevo | 0 |

El fixture usa un fetcher controlado para aislar worker/persistencia; no mide latencia real de red, throttling de un proveedor, RSS del contenedor ni un máximo absoluto de heap. Otra prueba de integración recorre HTTP real, apaga/reconstruye el servicio con las mismas bases/llaves y comprueba webhook, propietario, deadline y repetición de la petición que estaba en vuelo. Las pruebas ordinarias incluyen un resultado de ~24 MiB, superior al límite v1.

## Recuperación y fallos

El deadline se fija al aceptar y nunca se reinicia. Después de adquirir el bloqueo exclusivo, v2 devuelve jobs no terminales a pending, incrementa generación/recuperaciones y los admite por los mismos slots globales. Carga la última pila confirmada y sus inputs. V1 conserva PROCESS_INTERRUPTED.

Se reserva cada intento físico antes de HTTP. Una caída entre reservar y confirmar puede consumir un intento sin respuesta guardada; la petición en vuelo puede repetirse. No se promete exactamente una petición externa. El trabajo ya confirmado no se vuelve a emitir ni solicitar. Los cambios de configuración no extienden el deadline persistido. Un checkpoint incompatible termina con `UNSUPPORTED_CHECKPOINT_VERSION`.

Un error de almacenamiento detiene ese runner y desactiva readiness para nuevas admisiones; los datos anteriores siguen durables. Corregir disco/permisos/llaves y reiniciar para recuperar. No existe una API de pausa/reanudar trabajo: detener seguimiento en playground sólo desconecta el lector. `/health/ready`, logs y `GET /v2/executions/{id}` permiten revisar salud, progreso, presupuestos y contador de recuperación.

La publicación terminal y su webhook son una transacción. Failed no borra fragmentos existentes: el cliente debe consultar el terminal y tratar como incompletos los contenedores sin cierre. La retención eventualmente devuelve 410 para chunks/tree/export.

## Migración y rollback

1. Detener limpiamente la instancia anterior. Confirmar que ningún proceso usa el directorio de datos.
2. Respaldar **todo** el volumen, configuración y llaves en una ubicación protegida. No copiar únicamente engine.db mientras hay actividad WAL.
3. Construir/iniciar el binario nuevo. La migración 000002 es aditiva; valida checksums, conserva filas v1 y les asigna api_version=1.
4. Confirmar `/health/ready`, crear un job v2 controlado, observar chunks antes del fin y verificar su terminal/export.

No convertir resultados v1 ni ejecutar un backfill. No arrancar el binario anterior sobre una base con checkpoints v2. Para rollback, detener v2 y restaurar el respaldo previo y sus llaves; esto también descarta el trabajo creado después del respaldo. No ejecutar la migración down sobre datos que deban conservarse.

Ver [contrato de consumo](api-v2-guide.md) y [spec fuente de verdad](specs/005-api-execution-engine-v2.md).
