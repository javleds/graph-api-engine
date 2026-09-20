import { renderJSON } from "./json-viewer.js";

export function createV2Viewer(host) {
  const completed = element("section", "v2-results");
  const diagnostic = document.createElement("details");
  diagnostic.append(element("summary", "", "Diagnóstico: árbol incremental y chunks"));
  const legacyHost = element("div", "");
  diagnostic.append(legacyHost);
  host.append(completed, diagnostic);
  const legacy = createChunkViewer(legacyHost);
  const results = createResultsViewer(completed, async (nodeId) => {
    if (latest) await legacy.update(...latest);
    diagnostic.open = true;
    return legacy.openNode(nodeId);
  });
  let latest;
  diagnostic.addEventListener("toggle", () => {
    if (diagnostic.open && latest?.[1].v2?.resultsEnabled) {
      legacy.update(...latest).catch(() => {});
    }
  });
  return {
    async update(url, status, signal) {
      latest = [url, status, signal];
      host.hidden = false;
      const enabled = Boolean(status.v2?.resultsEnabled);
      completed.hidden = !enabled;
      diagnostic.querySelector("summary").hidden = !enabled;
      if (!enabled) {
        diagnostic.open = true;
        return legacy.update(url, status, signal);
      }
      await results.update(url, status, signal);
      return false;
    },
    cancel() { legacy.cancel(); results.cancel(); },
    reset() {
      latest = undefined;
      legacy.reset(); results.reset();
      diagnostic.open = false;
      host.hidden = true;
    },
  };
}

// Only a page of tree nodes and the latest 200 chunk envelopes stay in the DOM.
function createChunkViewer(host) {
  let generation = 0;
  let controller;
  let base = "";
  let cursor = "0";
  let chunks = [];
  let selectedRaw = "";
  let parent = "";
  let ancestors = [];
  let treeCursor = "0";
  let treeNext = "0";
  let treeMore = false;
  let mode = "tree";
  const toolbar = element("div", "mode-switch");
  toolbar.setAttribute("role", "group");
  toolbar.setAttribute("aria-label", "Vista de fragmentos");
  const modes = new Map();
  for (const [key, title] of [
    ["tree", "Árbol"],
    ["chunks", "Chunks"],
    ["raw", "Raw JSON"],
  ]) {
    const button = action(title, () => showMode(key));
    modes.set(key, button);
    toolbar.append(button);
  }
  const summary = element("p", "hint");
  const error = element("p", "message");
  error.setAttribute("role", "alert");
  error.hidden = true;
  const tree = element("div", "v2-tree");
  const navigation = element("div", "actions");
  const up = action("Subir al padre", () => {
    parent = ancestors.pop() ?? "";
    treeCursor = "0";
    refreshTree();
  });
  const first = action("Primera página", () => {
    treeCursor = "0";
    refreshTree();
  });
  const next = action("Siguiente página", () => {
    treeCursor = treeNext;
    refreshTree();
  });
  const refresh = action("Actualizar rama", refreshTree);
  navigation.append(up, first, next, refresh);
  const nodes = element("ul", "v2-nodes");
  tree.append(navigation, nodes);
  const list = element("div", "v2-chunks");
  list.setAttribute("aria-label", "Últimos 200 fragmentos");
  const raw = document.createElement("textarea");
  raw.readOnly = true;
  raw.spellcheck = false;
  raw.setAttribute("aria-label", "Fragmento JSON original (solo lectura)");
  const copy = action("Copiar fragmento", async () => {
    if (!selectedRaw) return;
    try {
      await navigator.clipboard.writeText(selectedRaw);
    } catch {
      showMode("raw");
      raw.focus();
      raw.select();
    }
  });
  copy.disabled = true;
  const download = document.createElement("a");
  download.className = "secondary";
  download.textContent = "Descargar NDJSON";
  download.setAttribute("download", "execution.ndjson");
  const actions = element("div", "actions");
  actions.append(copy, download);
  host.append(toolbar, summary, error, tree, list, raw, actions);

  function showMode(value) {
    mode = value;
    tree.hidden = value !== "tree";
    list.hidden = value !== "chunks";
    raw.hidden = value !== "raw";
    for (const [key, button] of modes)
      button.setAttribute("aria-pressed", String(key === value));
  }
  function cancel() {
    generation++;
    controller?.abort();
  }
  function reset() {
    cancel();
    base = "";
    cursor = "0";
    chunks = [];
    selectedRaw = "";
    raw.value = "";
    parent = "";
    ancestors = [];
    treeCursor = "0";
    nodes.replaceChildren();
    list.replaceChildren();
    copy.disabled = true;
    error.hidden = true;
    host.hidden = true;
    showMode("tree");
  }
  async function read(url, signal) {
    const response = await fetch(url, { signal, cache: "no-store" });
    const text = await response.text();
    if (!response.ok)
      throw new Error(
        `Consulta de fragmentos: HTTP ${response.status}. ${text.slice(0, 300)}`,
      );
    return { data: JSON.parse(text), text };
  }
  function select(item) {
    selectedRaw = item.raw;
    raw.value = selectedRaw;
    copy.disabled = false;
    showMode("raw");
  }
  function renderChunks() {
    list.replaceChildren();
    for (const chunk of chunks) {
      const button = action(
        `#${chunk.data.sequence} · ${chunk.data.type} · ${chunk.data.operation ?? chunk.data.status ?? ""}`,
        () => select(chunk),
      );
      list.append(button);
    }
  }
  async function loadTree(signal, current) {
    const query = new URLSearchParams({ after: treeCursor, limit: "100" });
    if (parent) query.set("parentId", parent);
    const { data, text } = await read(`${base}/tree?${query}`, signal);
    if (current !== generation) return;
    treeNext = data.nextCursor;
    treeMore = data.hasMore;
    up.disabled = !parent;
    first.disabled = treeCursor === "0";
    next.disabled = !treeMore;
    nodes.replaceChildren();
    const originals = arraySources(text, "nodes");
    for (let index = 0; index < (data.nodes ?? []).length; index++) {
      // Preserve number lexemes in labels, including integers beyond Number.MAX_SAFE_INTEGER.
      let numericValue;
      const node = JSON.parse(originals[index], (key, value, context) => {
        if (typeof value !== "number") return value;
        if (key === "value")
          numericValue = context?.source ?? "número (consultar Raw)";
        return context?.source ?? String(value);
      });
      const row = document.createElement("li");
      const label = node.index != null ? `[${node.index}]` : node.key || '""';
      const container = node.kind === "object" || node.kind === "array";
      const value = container
        ? node.kind
        : numericValue !== undefined
          ? numericValue
          : JSON.stringify(node.value);
      const title = `${label}: ${value ?? "pendiente"} · ${node.status || "pendiente"}`;
      if (container) {
        row.append(
          action(title, () => {
            ancestors.push(parent);
            parent = node.nodeId;
            treeCursor = "0";
            refreshTree();
          }),
        );
      } else row.textContent = title;
      nodes.append(row);
    }
    if (!data.nodes?.length)
      nodes.append(
        element(
          "li",
          "hint",
          data.terminal
            ? "Sin nodos en esta rama."
            : "Esperando nodos confirmados…",
        ),
      );
  }
  async function refreshTree() {
    if (!base) return;
    controller?.abort();
    controller = new AbortController();
    const current = ++generation;
    try {
      await loadTree(controller.signal, current);
      if (current === generation) error.hidden = true;
    } catch (cause) {
      if (current !== generation || cause.name === "AbortError") return;
      error.textContent = cause.message;
      error.hidden = false;
    }
  }
  async function update(url, status, signal) {
    if (base && base !== url) reset();
    base = url;
    host.hidden = false;
    download.href = `${base}/export`;
    const current = generation;
    const { data, text } = await read(
      `${base}/chunks?after=${cursor}&limit=100`,
      signal,
    );
    if (signal.aborted || current !== generation) return true;
    const originals = arraySources(text, "chunks");
    const known = new Set(chunks.map((chunk) => chunk.data.sequence));
    for (let index = 0; index < (data.chunks ?? []).length; index++) {
      const chunk = data.chunks[index];
      if (!known.has(chunk.sequence)) {
        chunks.push({ data: chunk, raw: originals[index] });
        known.add(chunk.sequence);
      }
    }
    chunks = chunks.slice(-200);
    cursor = data.nextCursor;
    renderChunks();
    if (!selectedRaw && chunks.length) {
      selectedRaw = chunks[0].raw;
      raw.value = selectedRaw;
      copy.disabled = false;
    }
    const metadata = status.v2 ?? status.result ?? status;
    summary.textContent = `Cursor ${cursor} / ${data.lastSequence} · ${metadata.publishedBytes ?? 0} bytes publicados · ${metadata.recoveries ?? 0} recuperaciones · ${chunks.length} fragmentos en pantalla`;
    if (mode === "tree") await loadTree(signal, current);
    return Boolean(data.hasMore);
  }
  reset();
  return { update, reset, cancel, async openNode(nodeId) {
    parent = nodeId;
    ancestors = [];
    treeCursor = "0";
    showMode("tree");
    await refreshTree();
  } };
}

function createResultsViewer(host, browse) {
  let base = "";
  let generation = 0;
  let pageRequest;
  let detailRequest;
  let cutoff = "0";
  let latestSequence = "0";
  let after = "0";
  let nextCursor = "0";
  let history = [];
  let initialized = false;
  const summary = element("p", "hint");
  const webhookSummary = element("p", "hint");
  summary.setAttribute("aria-live", "polite");
  const notice = action("Ver resultados nuevos", () => {
    cutoff = latestSequence;
    after = "0";
    history = [];
    loadPage();
  });
  notice.hidden = true;
  const previous = action("Resultados anteriores", () => {
    after = history.pop() ?? "0";
    loadPage();
  });
  const next = action("Resultados siguientes", () => {
    history.push(after);
    after = nextCursor;
    loadPage();
  });
  const nav = element("div", "actions");
  nav.append(previous, next, notice);
  const rows = element("ul", "v2-result-list");
  rows.setAttribute("aria-label", "Resultados completos");
  const error = element("p", "message");
  error.setAttribute("role", "alert");
  const retry = action("Reintentar consulta de resultados", () => loadPage());
  retry.hidden = true;
  const detail = element("section", "v2-result-detail");
  detail.setAttribute("aria-label", "Detalle del resultado");
  host.append(element("h3", "", "Resultados"), summary, webhookSummary, nav, error, retry, rows, detail);

  function cancel() {
    generation++;
    pageRequest?.abort();
    detailRequest?.abort();
  }
  function reset() {
    cancel();
    base = "";
    initialized = false;
    cutoff = latestSequence = after = nextCursor = "0";
    history = [];
    rows.replaceChildren();
    detail.replaceChildren();
    error.textContent = "";
    retry.hidden = true;
    previous.disabled = next.disabled = true;
    notice.hidden = true;
  }
  async function loadPage(signal) {
    pageRequest?.abort();
    const request = new AbortController();
    pageRequest = request;
    const current = generation;
    const abort = () => request.abort();
    signal?.addEventListener("abort", abort, { once: true });
    previous.disabled = next.disabled = true;
    try {
      const query = new URLSearchParams({ after, limit: "50", through: cutoff });
      const response = await fetch(`${base}/results?${query}`, { signal: request.signal, cache: "no-store" });
      if (!response.ok) throw new Error(`Resultados: HTTP ${response.status}`);
      const data = await response.json();
      if (current !== generation || request.signal.aborted) return;
      // A zero cutoff means latest at read time; a result may close after status was read.
      cutoff = data.lastSequence ?? cutoff;
      if (BigInt(cutoff) > BigInt(latestSequence)) latestSequence = cutoff;
      error.textContent = "";
      retry.hidden = true;
      const items = data.results ?? [];
      initialized = initialized || items.length > 0;
      nextCursor = data.nextCursor;
      previous.disabled = history.length === 0;
      next.disabled = !data.hasMore;
      rows.replaceChildren();
      for (const item of items) {
        const row = document.createElement("li");
        row.dataset.nodeId = item.nodeId;
        row.append(action(`${item.root || item.path || "Nodo"}${item.index != null ? ` [${item.index}]` : ""} · ${item.status}`, () => select(item)));
        if (item.errorSummary?.total) row.append(element("p", "hint", `${item.errorSummary.total} errores en esta cadena`));
        rows.append(row);
      }
      if (!items.length) rows.append(element("li", "hint", data.terminal ? "No hay cadenas completas. Una colección vacía o una ejecución interrumpida puede no producir resultados." : "Esperando la primera cadena completa…"));
      notice.hidden = latestSequence === cutoff;
    } catch (cause) {
      if (current === generation && cause.name !== "AbortError") {
        error.textContent = cause.message;
        retry.hidden = false;
      }
    } finally {
      signal?.removeEventListener("abort", abort);
    }
  }
  async function select(item) {
    detailRequest?.abort();
    const request = new AbortController();
    detailRequest = request;
    const current = generation;
    const url = `${base}/results/${encodeURIComponent(item.nodeId)}`;
    const heading = element("h4", "", `${item.root || item.path || "Nodo"} · ${item.status}`);
    const download = element("a", "secondary", "Descargar resultado JSON");
    download.href = url;
    download.download = `${item.nodeId}.json`;
    const controls = element("div", "actions");
    controls.append(download);
    const message = element("p", "hint", "Cargando cadena completa…");
    detail.replaceChildren(heading, controls, message);
    try {
      const response = await fetch(url, { signal: request.signal, cache: "no-store" });
      if (!response.ok) throw new Error(`Resultado: HTTP ${response.status}`);
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let bytes = 0;
      let raw = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        bytes += value.byteLength;
        if (bytes > 1024 * 1024) {
          await reader.cancel();
          if (current !== generation || request.signal.aborted) return;
          message.textContent = "El resultado supera 1 MiB. Puedes explorar sus ramas o descargar el JSON completo.";
          controls.append(action("Explorar ramas del resultado", () => browse(item.nodeId)));
          return;
        }
        raw += decoder.decode(value, { stream: true });
      }
      raw += decoder.decode();
      if (current !== generation || request.signal.aborted) return;
      if (JSON.parse(raw).complete !== true) throw new Error("La transferencia no contiene un resultado completo. Vuelve a abrirlo.");
      message.textContent = "Cadena terminada. Su contenido no cambiará durante la ejecución.";
      const tree = element("div", "json-tree");
      const fallback = renderJSON(tree, raw);
      if (fallback) message.textContent = fallback;
      const original = document.createElement("details");
      original.append(element("summary", "", "JSON original"));
      const textarea = document.createElement("textarea");
      textarea.readOnly = true;
      textarea.value = raw;
      textarea.setAttribute("aria-label", "Resultado JSON original (solo lectura)");
      original.append(textarea);
      controls.append(action("Copiar resultado", async () => {
        try { await navigator.clipboard.writeText(raw); }
        catch { original.open = true; textarea.focus(); textarea.select(); }
      }));
      detail.append(tree, original);
    } catch (cause) {
      if (current === generation && !request.signal.aborted) message.textContent = cause.message;
    }
  }
  async function update(url, status, signal) {
    if (base && base !== url) reset();
    base = url;
    const observed = status.v2.lastResultSequence ?? "0";
    if (BigInt(observed) > BigInt(latestSequence)) latestSequence = observed;
    summary.textContent = `${status.v2.resultCount ?? latestSequence} cadenas completas · ${status.status}`;
    const deliveries = status.nodeWebhookSummary;
    webhookSummary.hidden = !deliveries;
    if (deliveries) webhookSummary.textContent = `Webhooks de nodos: ${deliveries.pending} pendientes · ${deliveries.delivered} entregados · ${deliveries.failed} fallidos. La entrega no cambia el estado del resultado.`;
    notice.hidden = !initialized || latestSequence === cutoff;
    if (!notice.hidden) notice.textContent = `Hay ${BigInt(latestSequence) - BigInt(cutoff)} resultados nuevos. Ver resultados nuevos`;
    if (!initialized) {
      cutoff = latestSequence;
      await loadPage(signal);
    }
  }
  reset();
  return { update, reset, cancel };
}

function element(tag, className, text) {
  const node = document.createElement(tag);
  node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}
function action(title, callback) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "subtle";
  button.textContent = title;
  button.addEventListener("click", callback);
  return button;
}

// Keep exact server JSON bytes for each envelope rather than reserializing numbers.
function arraySources(text, field) {
  const match = new RegExp(`"${field}"\\s*:\\s*\\[`).exec(text);
  if (!match) return [];
  const result = [];
  let start = match.index + match[0].length;
  let depth = 0;
  let quoted = false;
  let escaped = false;
  for (let index = start; index < text.length; index++) {
    const char = text[index];
    if (quoted) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === '"') quoted = false;
      continue;
    }
    if (char === '"') quoted = true;
    else if (char === "{" || char === "[") depth++;
    else if (char === "]" && depth === 0) {
      const value = text.slice(start, index).trim();
      if (value) result.push(value);
      return result;
    } else if (char === "}" || char === "]") depth--;
    else if (char === "," && depth === 0) {
      result.push(text.slice(start, index).trim());
      start = index + 1;
    }
  }
  return result;
}
