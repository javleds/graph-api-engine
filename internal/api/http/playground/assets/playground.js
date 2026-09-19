import { initializeInputEditor } from "./input-editor.js";
import { createV2Viewer } from "./v2-viewer.js";
import { renderJSON } from "./json-viewer.js";

const $ = (id) => document.getElementById(id);
const elements = Object.fromEntries(
  [
    "request",
    "execute",
    "clear",
    "stop",
    "cancel-execution",
    "resume",
    "execution-id",
    "status",
    "tracking-state",
    "execution-meta",
    "progress",
    "message",
    "webhook",
    "response-label",
    "response-empty",
    "response-raw",
    "copy",
    "debug",
  ].map((id) => [id, $(id)]),
);
const version = () => $("execution-version").value;
const apiBase = () =>
  version() === "2"
    ? "/playground/api/v2/executions"
    : "/playground/api/executions";
initializeInputEditor(() =>
  version() === "2"
    ? "/playground/api/v2/validate"
    : "/playground/api/validate",
);
const v2Viewer = createV2Viewer($("v2-output"));
const labels = {
  pending: "◷ Pendiente",
  running: "◷ En ejecución",
  completed: "✓ Completado",
  completed_with_errors: "△ Completado con errores",
  failed: "× Fallido",
};
const counterLabels = {
  successfulRootCalls: "raíces completas",
  partialRootCalls: "raíces parciales",
  failedRootCalls: "raíces fallidas",
  logicalRequests: "requests lógicos",
  httpAttempts: "intentos HTTP",
  cacheHits: "aciertos de caché",
  coalescedRequests: "requests compartidos",
  retries: "reintentos",
  completedRootItems: "items completos",
  nodeFailures: "errores de nodos",
  skippedSubtrees: "subárboles omitidos",
};
const example = {
  definition: {
    version: 1,
    api: {
      baseUrl: "https://example.com/api",
      limits: { requests: 100, per: "1m" },
    },
    calls: [
      {
        name: "items",
        type: "collection",
        select: "data",
        request: { method: "GET", path: "/items" },
      },
    ],
  },
};
let generation = 0;
let controller = null;
let timer = null;
let tracking = false;
let currentID = "";
let currentStatus = "";
let cancelling = false;
let rawResponse = null;
let treeAvailable = false;
const exchanges = new Map();

function message(text = "") {
  elements.message.textContent = text;
  elements.message.hidden = !text;
}
function setTracking(active, text) {
  tracking = active;
  $("execution-version").disabled = active;
  elements.execute.disabled = active;
  elements.clear.disabled = active;
  elements.stop.disabled = !active;
  if ($("refresh-webhook")) $("refresh-webhook").disabled = active;
  elements["tracking-state"].textContent = text;
  updateCancelButton();
}
function updateCancelButton() {
  elements["cancel-execution"].disabled =
    cancelling || !currentID || !["pending", "running"].includes(currentStatus);
  elements["cancel-execution"].textContent = cancelling
    ? "Abortando ejecución…"
    : "Abortar ejecución";
}
function beginSession() {
  generation += 1;
  controller?.abort();
  v2Viewer.cancel();
  clearTimeout(timer);
  controller = new AbortController();
  cancelling = false;
  updateCancelButton();
  return generation;
}
function stopTracking() {
  beginSession();
  setTracking(false, "Seguimiento detenido; el job continúa en el servidor.");
}
function resetResponse() {
  v2Viewer.reset();
  treeAvailable = false;
  $("response-modes").hidden = true;
  $("response-tree").hidden = true;
  $("response-tree").replaceChildren();
  rawResponse = null;
  elements["response-raw"].value = "";
  elements["response-raw"].hidden = true;
  elements["response-empty"].hidden = false;
  elements.copy.disabled = true;
  elements["response-label"].textContent =
    "El resultado aparecerá cuando termine el job.";
}
function displayResponse(raw, label) {
  rawResponse = raw;
  elements["response-raw"].value = raw;
  elements["response-raw"].hidden = false;
  elements["response-empty"].hidden = true;
  elements.copy.disabled = false;
  const fallback = renderJSON($("response-tree"), raw);
  treeAvailable = !fallback;
  $("response-modes").hidden = false;
  $("mode-tree").disabled = !treeAvailable;
  setResponseMode(treeAvailable ? "tree" : "raw");
  elements["response-label"].textContent = fallback
    ? `${label}. ${fallback}`
    : label;
}
function setResponseMode(mode) {
  const tree = mode === "tree" && treeAvailable;
  $("response-tree").hidden = !tree;
  elements["response-raw"].hidden = tree;
  $("mode-tree").setAttribute("aria-pressed", String(tree));
  $("mode-raw").setAttribute("aria-pressed", String(!tree));
}
function renderDebug() {
  const open = new Set(
    [...elements.debug.querySelectorAll("details[open]")].map(
      (node) => node.dataset.key,
    ),
  );
  elements.debug.replaceChildren();
  for (const [key, exchange] of exchanges) {
    const details = document.createElement("details");
    details.className = "debug-entry";
    details.dataset.key = key;
    details.open = open.has(key);
    const summary = document.createElement("summary");
    summary.textContent = `${exchange.title} · ${exchange.status ?? "texto original"}`;
    const meta = document.createElement("div");
    meta.className = "debug-meta";
    meta.textContent = [
      exchange.method && `${exchange.method} ${exchange.url}`,
      exchange.duration != null && `${exchange.duration} ms`,
      ...(exchange.headers ?? []),
    ]
      .filter(Boolean)
      .join("\n");
    const body = document.createElement("pre");
    body.textContent = exchange.body;
    details.append(summary, meta, body);
    elements.debug.append(details);
  }
}
async function request(key, title, url, options, session) {
  const start = performance.now();
  let response;
  let body = "";
  try {
    response = await fetch(url, {
      ...options,
      signal: controller.signal,
      cache: "no-store",
    });
    body = await response.text();
  } catch (error) {
    if (session !== generation || error.name === "AbortError") throw error;
    exchanges.set(key, {
      title,
      method: options.method ?? "GET",
      url,
      status: "Error de transporte",
      duration: Math.round(performance.now() - start),
      body: error.message,
    });
    renderDebug();
    throw new Error(
      "No se pudo completar la consulta. El estado del job es desconocido; puedes reanudar por ID. El POST no se repetirá automáticamente.",
    );
  }
  if (session !== generation)
    throw new DOMException("Sesión reemplazada", "AbortError");
  const headers = ["X-Request-Id", "Location", "Retry-After"].flatMap((name) =>
    response.headers.has(name)
      ? [`${name}: ${response.headers.get(name)}`]
      : [],
  );
  exchanges.set(key, {
    title,
    method: options.method ?? "GET",
    url,
    status: `HTTP ${response.status}`,
    duration: Math.round(performance.now() - start),
    body,
    headers,
  });
  renderDebug();
  if (!response.ok) {
    displayResponse(body, `Error HTTP ${response.status} · ${title}`);
    throw new Error(
      `${title}: HTTP ${response.status}. Revisa el body y los headers en Depuración HTTP.`,
    );
  }
  return body;
}
function parsedObject(raw) {
  const value = JSON.parse(raw);
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error("La respuesta de la API no es un objeto JSON válido.");
  return value;
}
function addMeta(label, value) {
  const item = document.createElement("div");
  const term = document.createElement("dt");
  const detail = document.createElement("dd");
  term.textContent = label;
  detail.textContent = value;
  item.append(term, detail);
  elements["execution-meta"].append(item);
}
function formatDate(value) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString("es");
}
function renderStatus(status) {
  currentStatus = status.status ?? "";
  elements.status.textContent =
    status.status === "failed" && status.error?.code === "EXECUTION_CANCELLED"
      ? "× Cancelada"
      : (labels[status.status] ?? `? ${status.status ?? "Estado desconocido"}`);
  updateCancelButton();
  elements.status.dataset.status = status.status ?? "";
  elements["execution-meta"].replaceChildren();
  addMeta("ID de ejecución", currentID);
  addMeta("Creación", formatDate(status.createdAt));
  addMeta("Inicio", formatDate(status.startedAt));
  addMeta("Fin", formatDate(status.finishedAt));
  const start = Date.parse(status.startedAt ?? status.createdAt);
  const end = status.finishedAt ? Date.parse(status.finishedAt) : Date.now();
  addMeta(
    "Tiempo transcurrido",
    Number.isFinite(start) && Number.isFinite(end)
      ? `${Math.max(0, (end - start) / 1000).toFixed(1)} s`
      : "—",
  );
  addMeta("Última actualización", new Date().toLocaleTimeString("es"));
  elements.progress.replaceChildren();
  for (const [key, value] of Object.entries(status.progress ?? {})) {
    const item = document.createElement("span");
    item.className = "counter";
    const count = document.createElement("strong");
    count.textContent = value;
    item.append(count, counterLabels[key] ?? key);
    elements.progress.append(item);
  }
  renderWebhook(status);
}
function renderWebhook(status) {
  elements.webhook.replaceChildren();
  elements.webhook.hidden = !status.webhook;
  if (!status.webhook) return;
  const label = document.createElement("span");
  label.textContent = `Webhook: ${status.webhook.status} · ${status.webhook.attempts} intentos`;
  elements.webhook.append(label);
  if (
    ["completed", "completed_with_errors", "failed"].includes(status.status)
  ) {
    const button = document.createElement("button");
    button.className = "secondary";
    button.id = "refresh-webhook";
    button.disabled = tracking;
    button.textContent = "Actualizar webhook";
    button.addEventListener("click", () => startTracking(currentID, false));
    elements.webhook.append(button);
  }
}
function validExecutionID(id) {
  return (
    typeof id === "string" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)
  );
}
function clearStatus() {
  currentStatus = "";
  updateCancelButton();
  elements["execution-meta"].replaceChildren();
  elements.progress.replaceChildren();
  elements.webhook.hidden = true;
  elements.status.textContent = "○ Sin estado disponible";
  delete elements.status.dataset.status;
}
function fail(error, session) {
  if (session !== generation || error.name === "AbortError") return;
  if (!currentID) {
    elements.status.textContent = "× Sin ejecución confirmada";
    elements.status.dataset.status = "failed";
  }
  setTracking(false, "Seguimiento pausado");
  message(error.message);
}
async function poll(session, loadResult = true) {
  try {
    const raw = await request(
      "status",
      "Último estado",
      `${apiBase()}/${encodeURIComponent(currentID)}`,
      {},
      session,
    );
    const status = parsedObject(raw);
    renderStatus(status);
    if (version() === "2") {
      elements["response-empty"].hidden = true;
      elements["response-label"].textContent =
        "Fragmentos confirmados en disco · el árbol se consulta por ramas.";
      const more = await v2Viewer.update(
        `${apiBase()}/${encodeURIComponent(currentID)}`,
        status,
        controller.signal,
      );
      if (session !== generation) return;
      const terminal = [
        "completed",
        "completed_with_errors",
        "failed",
      ].includes(status.status);
      setTracking(
        !terminal || more,
        terminal && !more ? "Ejecución terminada" : "Recibiendo fragmentos…",
      );
      if (!terminal || more)
        timer = setTimeout(() => poll(session), more ? 0 : 1000);
      return;
    }
    if (status.status === "failed") {
      displayResponse(
        raw,
        status.error?.code === "EXECUTION_CANCELLED"
          ? "Ejecución cancelada · respuesta terminal de estado"
          : "Ejecución fallida · respuesta terminal de estado",
      );
      setTracking(false, "Ejecución terminada");
      return;
    }
    if (["completed", "completed_with_errors"].includes(status.status)) {
      if (loadResult) {
        elements["tracking-state"].textContent = "Obteniendo resultado…";
        const result = await request(
          "result",
          "Resultado",
          `${apiBase()}/${encodeURIComponent(currentID)}/result`,
          {},
          session,
        );
        displayResponse(
          result,
          status.status === "completed"
            ? "Resultado completo · envelope de la API"
            : "Resultado parcial · incluye errores y resumen por raíz",
        );
      }
      setTracking(false, "Ejecución terminada");
      return;
    }
    if (!["pending", "running"].includes(status.status))
      throw new Error(
        "La API devolvió un estado desconocido. Revisa la respuesta de estado.",
      );
    setTracking(true, "Consultando estado cada segundo…");
    timer = setTimeout(() => poll(session, loadResult), 1000);
  } catch (error) {
    fail(error, session);
  }
}
async function cancelExecution() {
  if (elements["cancel-execution"].disabled) return;
  const session = beginSession();
  cancelling = true;
  message();
  setTracking(false, "Solicitando cancelación al servidor…");
  try {
    const raw = await request(
      "cancel",
      "Cancelación de ejecución",
      `${apiBase()}/${encodeURIComponent(currentID)}/cancel`,
      { method: "POST" },
      session,
    );
    renderStatus(parsedObject(raw));
    cancelling = false;
    setTracking(true, "Consultando cierre de ejecución…");
    await poll(session);
  } catch (error) {
    if (session !== generation || error.name === "AbortError") return;
    cancelling = false;
    fail(error, session);
  }
}
function startTracking(id, loadResult = true) {
  if (!validExecutionID(id)) {
    message("El ID de ejecución debe ser un UUID válido.");
    elements["execution-id"].focus();
    return;
  }
  const session = beginSession();
  if (id !== currentID) {
    clearStatus();
    resetResponse();
    exchanges.clear();
    renderDebug();
  }
  currentID = id;
  elements["execution-id"].value = id;
  message();
  setTracking(true, "Consultando ejecución…");
  poll(session, loadResult);
}
async function execute() {
  if (tracking) return;
  const raw = elements.request.value;
  try {
    JSON.parse(raw);
  } catch (error) {
    message(`JSON inválido: ${error.message}`);
    elements.request.focus();
    return;
  }
  const session = beginSession();
  currentID = "";
  currentStatus = "";
  elements["execution-id"].value = "";
  resetResponse();
  elements["execution-meta"].replaceChildren();
  elements.progress.replaceChildren();
  elements.webhook.hidden = true;
  elements.status.textContent = "◷ Enviando request";
  elements.status.dataset.status = "pending";
  message();
  setTracking(true, "Creando ejecución…");
  const headers = { "Content-Type": "application/json" };
  const idempotency = $("idempotency").value;
  if (idempotency) headers["Idempotency-Key"] = idempotency;
  exchanges.clear();
  exchanges.set("request", {
    title: "Request enviado",
    method: "POST",
    url: apiBase(),
    headers: Object.entries(headers).map(([key, value]) => `${key}: ${value}`),
    body: raw,
  });
  renderDebug();
  try {
    const body = await request(
      "create",
      "Creación",
      apiBase(),
      { method: "POST", headers, body: raw },
      session,
    );
    const created = parsedObject(body);
    if (!validExecutionID(created.id))
      throw new Error("La API no devolvió un ID de ejecución válido.");
    currentID = created.id;
    elements["execution-id"].value = currentID;
    renderStatus(created);
    await poll(session);
  } catch (error) {
    fail(error, session);
  }
}
$("example").addEventListener("click", () => {
  elements.request.value = JSON.stringify(example, null, 2);
  elements.request.dispatchEvent(new Event("input"));
  message(
    "Ejemplo cargado. Sustituye example.com por un proveedor permitido que devuelva JSON.",
  );
});
elements.execute.addEventListener("click", execute);
elements.stop.addEventListener("click", stopTracking);
elements["cancel-execution"].addEventListener("click", cancelExecution);
$("resume-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const id = elements["execution-id"].value.trim();
  if (id) startTracking(id);
});
elements.clear.addEventListener("click", () => {
  if (tracking) return;
  beginSession();
  elements.request.value = "";
  elements.request.dispatchEvent(new Event("input"));
  $("idempotency").value = "";
  elements["execution-id"].value = "";
  currentID = "";
  currentStatus = "";
  exchanges.clear();
  renderDebug();
  resetResponse();
  elements["execution-meta"].replaceChildren();
  elements.progress.replaceChildren();
  elements.webhook.hidden = true;
  elements.status.textContent = "○ Sin ejecución";
  delete elements.status.dataset.status;
  setTracking(false, "Listo para ejecutar");
  message();
});
elements.copy.addEventListener("click", async () => {
  if (rawResponse === null) return;
  try {
    await navigator.clipboard.writeText(rawResponse);
    elements["response-label"].textContent =
      "JSON original copiado al portapapeles.";
  } catch {
    setResponseMode("raw");
    elements["response-raw"].focus();
    elements["response-raw"].select();
    elements["response-label"].textContent =
      "Texto seleccionado. Usa Ctrl+C o ⌘C para copiar.";
  }
});
$("mode-tree").addEventListener("click", () => setResponseMode("tree"));
$("mode-raw").addEventListener("click", () => setResponseMode("raw"));
window.addEventListener("pagehide", () => {
  controller?.abort();
  v2Viewer.cancel();
  clearTimeout(timer);
});

$("execution-version").addEventListener("change", () => {
  beginSession();
  currentID = "";
  clearStatus();
  resetResponse();
  exchanges.clear();
  renderDebug();
  elements.request.dispatchEvent(new Event("input"));
  setTracking(false, "Listo para ejecutar");
});
