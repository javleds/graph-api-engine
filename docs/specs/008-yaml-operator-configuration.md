# 008 · Configuración de operador YAML

## Objetivo

Usar YAML como único formato de configuración del operador para `serve`, `migrate` y `validate --config`. Los documentos de ejecución, contratos y el archivo de principals permanecen en JSON.

## Contrato

- `--config` y `ENGINE_CONFIG` aceptan exclusivamente archivos con extensión `.yml` o `.yaml`.
- El archivo estándar de Compose es `config/config.yml`.
- La configuración conserva defaults, rechazo de campos desconocidos y validación de seguridad existentes.
- Se rechazan documentos múltiples, claves duplicadas, aliases, merge keys, claves no escalares y valores nulos.
- Los valores escalares admiten `${VARIABLE}`. Una variable faltante o vacía aborta la carga.
- Un valor formado únicamente por una variable se vuelve a interpretar como escalar YAML; una interpolación dentro de otro texto siempre produce un string.
- La interpolación no admite defaults, expresiones, inclusión de archivos ni sustitución de claves.

## Estrategia

El cargador valida el árbol YAML y lo normaliza a JSON interno antes de aplicar el decodificador JSON estricto existente. Así se preservan los tags actuales, los defaults, las duraciones y el rechazo de propiedades desconocidas sin duplicar la lógica de configuración.

## Validación

Cubrir carga del ejemplo, extensiones inválidas, duplicados, aliases, merges, múltiples documentos, null, campos desconocidos e interpolación de string, número, booleano, valor compuesto y variable ausente. Ejecutar pruebas Go, la suite de playground y una reconstrucción de Compose.

## Commits

1. `docs: specify yaml operator configuration`
2. `feat(config): load strict yaml operator settings`
3. `chore(config): migrate runtime configuration to yaml`
4. `docs: document yaml configuration and environment interpolation`
