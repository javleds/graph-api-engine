// Only a page of tree nodes and the latest 200 chunk envelopes stay in the DOM.
export function createV2Viewer(host) {
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
      const node = JSON.parse(originals[index], (_key, value, context) =>
        typeof value === "number" ? (context?.source ?? String(value)) : value,
      );
      const row = document.createElement("li");
      const label =
        node.key ??
        (node.index != null ? `[${node.index}]` : node.path || "raíz");
      const container = node.kind === "object" || node.kind === "array";
      const value = container
        ? node.kind
        : node.kind === "number"
          ? node.value
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
