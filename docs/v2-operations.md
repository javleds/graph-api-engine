# Operar v2

El servicio completo sigue en un único proceso y contenedor, puerto **8080**. `docker compose up -d --build` inicia API, worker y mantenimiento; el playground está en `http://localhost:8080/playground`. Seleccionar **v2 · Fragmentos** para publicación progresiva. V1 sigue disponible y seleccionado inicialmente.

## Recursos y límites

V2 guarda resultados y checkpoints en el mismo SQLite persistente. **1 GiB es capacidad máxima de resultado por job, no RAM reservada**. `executions.v2` configura:

| Campo | Default | Aplicación |
|---|---:|---|
| `maxPublishedBytes` | 1,073,741,824 | Bytes JSON de fragmentos por job, reservando 256 KiB para cierre. |
| `maxStoredBytes` | 8,589,934,592 | Bytes lógicos globales de chunks, proyección, inputs, checkpoints, eventos y entregas de nodo; reserva cierre por job activo. |
| `maxChunks` | 1,000,000 | Fragmentos por job, con un lugar reservado para terminal. |
| `maxNodes` | 1,000,000 | Nodos JSON publicados por job. |
| `maxPendingTasks` | 100,000 | Continuaciones persistidas pendientes por job. |

Las nuevas ejecuciones usan un scheduler durable con **ramas independientes concurrentes**. `executions.maxBranchGoroutines` limita tareas activas por ejecución, con mínimo efectivo de uno; `executions.maxConcurrent` limita jobs simultáneos (default cuatro). Se mantienen `fetch.maxConcurrentHTTPRequests`, las cuotas del proveedor/endpoint y los reintentos. La caché evita HTTP, pero no el recorrido y la persistencia. No se crea una goroutine por cada nodo: las expansiones son acotadas y los padres esperan dependencias sin ocupar un worker. La paginación upstream de cada llamada sigue secuencial, aunque sus elementos se procesan en paralelo.

Cada respuesta upstream sigue limitada por `fetch.maxResponseBytes` (default 4 MiB), profundidad y cantidad de valores. El heap depende de páginas decodificadas, ancestros necesarios, concurrencia, cache y fetch; JSON en memoria puede ocupar más que sus bytes en red. Aumentar paralelismo aumenta memoria activa, no reserva un grafo completo. Los nuevos lotes de publicación tienen topes de 1.024 pasos, 1.024 chunks o aproximadamente 1 MiB, además de fronteras de respuesta aceptada y cadena completa. Una llamada lenta no impide avanzar otras ramas independientes con capacidad disponible.

SQLite ocupa más espacio físico que el JSON: cifrado serializado, índices, proyección para consultas, páginas libres y WAL. `maxStoredBytes` es una cota **lógica**, no del tamaño de archivos. `storage.minimumFreeDiskBytes` sigue comprobándose antes de avanzar; dejar espacio para escrituras, cierre y backups. Borrar filas permite reutilizar páginas, pero no encoge inmediatamente el archivo. Los inputs aceptados se eliminan cuando ninguna continuación los necesita. Los resultados/proyección expiran por `storage.resultRetention`; la metadata dura `metadataRetention`.

Compose conserva el límite de contenedor de 1 GiB y `GOMEMLIMIT=768MiB`; son límites operativos generales del proceso, independientes del tamaño permitido para cada resultado.

## Eventos y receptores lentos

Cada cadena terminada confirma chunks finales, resultado, diagnósticos, evento `node.finished` y cambios de dependencias/checkpoint en la misma transacción. No se mantienen transacciones abiertas durante HTTP. El worker libera referencias de trabajo innecesarias; mantiene páginas y ancestros todavía usados. Los resultados nuevos se reconstruyen desde la proyección persistida, porque chunks de distintas ramas pueden intercalarse; no se duplica todo el subárbol en RAM. SQLite mantiene `synchronous=FULL`.

Un suscriptor independiente materializa webhooks en lotes acotados y rota entre ejecuciones. Congela/cifra el mensaje y avanza su cursor en una transacción. El dispatcher mantiene concurrencia acotada y reutiliza cada slot tan pronto termina, sin esperar al receptor más lento del lote. No requiere un broker y sigue siendo una sola instancia SQLite.

Para nuevas ejecuciones, el request admite `webhook.ordering: completion|source` (default `completion`) y `webhook.maxInFlight` (entero positivo, default `1`). El máximo efectivo se limita por `webhooks.maxConcurrent`, incluso al cambiar configuración y reiniciar. Con uno, no se admite la siguiente entrega hasta confirmar `2xx` o resolver definitivamente la anterior. Un reintento conserva su lugar lógico en la ventana, sin mantener ocupado un slot HTTP durante backoff. Otras ejecuciones pueden seguir enviando.

`source` espera las posiciones anteriores del request/colección; con varias entregas en vuelo ordena la admisión, no garantiza el orden de recepción o procesamiento remoto. Una respuesta `202` confirma recepción, no trabajo remoto terminado. La API sigue publicando por finalización en ambos modos. Las esperas consumen almacenamiento y plazo de entrega, nunca bloquean directamente el scheduler.

`maxPendingDeliveries` incluye reservas de terminales activos al admitir ejecuciones. Para admitir notificaciones de nodo se excluyen los terminales V2 que todavía esperan a esos nodos: de otro modo, una cola de capacidad uno se bloquearía a sí misma. Las entregas de nodo pendientes, en reintento o en vuelo sí consumen capacidad. Puede haber más filas pendientes persistidas que slots de admisión por esos terminales diferidos; su cantidad sigue limitada por la admisión de ejecuciones. Con capacidad uno, los nodos pueden esperar a que termine el engine antes de comenzar a enviarse. Una outbox llena deja eventos pendientes sin avanzar el cursor; no bloquea directamente al worker. También se respeta el presupuesto global de almacenamiento V2. Al acumular más resultados, el worker puede alcanzar su propio límite y terminar controladamente. No hay almacenamiento ilimitado.

`deliveryMaxAge` empieza en la fecha del evento, incluyendo la espera por capacidad. Los eventos vencidos se registran `failed` con `DELIVERY_DEADLINE_EXCEEDED` y cero intentos HTTP. `storage.resultRetention` debe ser mayor o igual a ese plazo; la configuración inválida impide iniciar. Los eventos sin proyectar se conservan para cerrar sus entregas antes de la purga. El payload se limita al envelope completo: excesos producen `WEBHOOK_PAYLOAD_TOO_LARGE`, sin afectar el resultado del engine.

Revisar separadamente estados de procesamiento y entrega. Con la nueva política, el terminal espera a resolver las entregas de nodo; las fallidas también cuentan como resueltas. Las ejecuciones antiguas conservan su política previa, donde un terminal podía preceder a un nodo reintentado. El resumen de notificaciones y su listado paginado permiten distinguir cola pendiente, entregas exitosas y fallos. Los receptores deben deduplicar `eventId`; los bytes firmados se reutilizan exactamente durante todos los intentos.

## Evidencia reproducible

Medición del scheduler concurrente, 2026-09-20, Apple M2, Go local y SQLite cifrado temporal. Mediana de cinco ejecuciones con ocho raíces independientes y ocho workers:

| Fixture controlado | V2 secuencial anterior | V2 concurrente |
|---|---:|---:|
| Fetch con 100 ms de latencia simulada por raíz | 841.42 ms | 122.49 ms |
| Respuestas inmediatas que simulan hits de caché | 19.74 ms | 16.48 ms |

La mejora de latencia fue aproximadamente 6.9× en ese fixture; no es una garantía para PokeAPI. El fixture de caché devuelve respuestas con `CacheHit=true` sin reservar intentos físicos: no mide lecturas reales de la base de caché, cuotas, warmup, red ni webhooks. El caso V1 del benchmark mide sólo `Engine.Execute`, sin persistencia final, y no es una comparación end-to-end equivalente. La concurrencia puede aumentar asignaciones y no garantiza acelerar trabajo exclusivamente de CPU/disco.

```sh
go test ./internal/worker -run '^$' -bench '^BenchmarkV2ExecutionSchedulers/(cold|cached)/(legacy|workers8)$' -benchtime=1x -count=5
```

La prueba grande del nuevo scheduler publicó 1,071,702,490 bytes con una reapertura intermedia, sin repetir páginas confirmadas, sin inputs retenidos al terminar y con un pico de `HeapAlloc` muestreado de 20,170,584 bytes. No es una cota de RSS ni una garantía para cualquier grafo/concurrencia. El tiempo observado fue 32.925 s; esta carga de publicación masiva no obtiene la mejora de las llamadas independientes con latencia.

```sh
make check
make playground-test
make test-v2-large
```

Referencia histórica del **runner secuencial anterior**, ejecutada el 2026-09-18, Go local y SQLite cifrado temporal, un job con 1,024 páginas, ocho escalares de aproximadamente 128 KiB por página. No representa una medición del nuevo scheduler:

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

El deadline se fija al aceptar y nunca se reinicia. Después de adquirir el bloqueo exclusivo, v2 devuelve jobs no terminales a pending, incrementa generación/recuperaciones y los admite por los mismos slots globales. Carga las tareas/dependencias del checkpoint confirmado y sus inputs; las confirmaciones de generaciones anteriores se rechazan. Los checkpoints anteriores conservan el runner secuencial y su pila. V1 conserva PROCESS_INTERRUPTED.

Se reserva cada intento físico antes de HTTP. Una caída entre reservar y confirmar puede consumir un intento sin respuesta guardada; la petición en vuelo puede repetirse. No se promete exactamente una petición externa. El trabajo ya confirmado no se vuelve a emitir ni solicitar. Los cambios de configuración no extienden el deadline persistido. Un checkpoint incompatible termina con `UNSUPPORTED_CHECKPOINT_VERSION`.

Un error de almacenamiento detiene ese runner y desactiva readiness para nuevas admisiones; los datos anteriores siguen durables. Corregir disco/permisos/llaves y reiniciar para recuperar. No existe una API de pausa/reanudar trabajo: detener seguimiento en playground sólo desconecta el lector. **Abortar ejecución** usa POST `/v2/executions/{id}/cancel`, publica un terminal `failed` con código `EXECUTION_CANCELLED` y detiene el contexto del árbol. Ese job no se recupera ni reanuda después de reiniciar; sus chunks confirmados se conservan hasta expirar. `/health/ready`, logs y `GET /v2/executions/{id}` permiten revisar salud, progreso, presupuestos y contador de recuperación.

La publicación terminal y su webhook son una transacción. Failed no borra fragmentos existentes: el cliente debe consultar el terminal y tratar como incompletos los contenedores sin cierre. La retención eventualmente devuelve 410 para chunks/tree/export.

## Migración y rollback

1. Detener limpiamente la instancia anterior. Confirmar que ningún proceso usa el directorio de datos.
2. Respaldar **todo** el volumen, configuración y llaves en una ubicación protegida. No copiar únicamente engine.db mientras hay actividad WAL.
3. Construir/iniciar el binario nuevo. La migración 000003 añadió resultados/eventos; 000004 (`concurrent_v2`) habilita la nueva ejecución y entrega concurrentes, preservando registros previos. Las ejecuciones anteriores conservan su runner, lectores y política de entrega: no reciben defaults retroactivamente.
4. Confirmar `/health/ready`, crear un job v2 controlado, observar un resultado completo antes del fin, verificar su JSON, evento de nodo y terminal. Revisar que el resumen de entrega distinga fallos del receptor y fallos del engine.

No convertir resultados v1 ni ejecutar un backfill. No arrancar el binario anterior sobre una base con checkpoints v2. Para rollback, detener v2 y restaurar el respaldo previo y sus llaves; esto también descarta el trabajo creado después del respaldo. No ejecutar la migración down sobre datos que deban conservarse.

Ver [contrato de consumo](api-v2-guide.md), [base v2](specs/005-api-execution-engine-v2.md) y [cadenas completas y eventos](specs/010-v2-complete-results-viewer.md).
