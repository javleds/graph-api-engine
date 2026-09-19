// Keep number tokens exactly as received, including integers beyond Number.MAX_SAFE_INTEGER.
class NumberToken {
  constructor(source) {
    this.source = source;
  }
}
function supportsNumberSource() {
  let supported = false;
  JSON.parse("9007199254740993", (_key, value, context) => {
    supported = context?.source === "9007199254740993";
    return value;
  });
  return supported;
}
export function renderJSON(container, raw) {
  container.replaceChildren();
  if (!supportsNumberSource())
    return "Este navegador no permite preservar todos los números en el visor. Usa el JSON original.";
  let value;
  try {
    value = JSON.parse(raw, (_key, item, context) =>
      typeof item === "number" ? new NumberToken(context.source) : item,
    );
  } catch {
    return "El body recibido no es JSON válido. Se muestra el texto original.";
  }
  const root = createNode(null, value);
  container.append(root);
  if (root.tagName === "DETAILS") root.open = true;
  return "";
}
function keyLabel(key) {
  const label = document.createElement("span");
  label.className = "json-key";
  label.textContent = key === null ? "" : `${JSON.stringify(key)}: `;
  return label;
}
function primitive(value) {
  const span = document.createElement("span");
  const type =
    value instanceof NumberToken
      ? "number"
      : value === null
        ? "null"
        : typeof value;
  span.className = `json-${type}`;
  span.textContent =
    value instanceof NumberToken ? value.source : JSON.stringify(value);
  return span;
}
function createNode(key, value) {
  if (
    value === null ||
    typeof value !== "object" ||
    value instanceof NumberToken
  ) {
    const row = document.createElement("div");
    row.className = "json-value";
    row.append(keyLabel(key), primitive(value));
    return row;
  }
  const array = Array.isArray(value);
  const keys = Object.keys(value);
  const details = document.createElement("details");
  details.className = "json-node";
  const summary = document.createElement("summary");
  const type = document.createElement("span");
  type.className = "json-collection";
  type.textContent = `${array ? "[ ]" : "{ }"} ${keys.length} ${array ? "elementos" : "propiedades"}`;
  summary.append(keyLabel(key), type);
  details.append(summary);
  let initialized = false;
  details.addEventListener("toggle", () => {
    if (!details.open || initialized) return;
    initialized = true;
    const children = document.createElement("div");
    children.className = "json-children";
    details.append(children);
    let cursor = 0;
    const more = document.createElement("button");
    more.className = "json-more";
    function appendPage() {
      more.remove();
      const end = Math.min(cursor + 100, keys.length);
      const fragment = document.createDocumentFragment();
      while (cursor < end) {
        const childKey = keys[cursor++];
        fragment.append(createNode(childKey, value[childKey]));
      }
      children.append(fragment);
      if (cursor < keys.length) {
        more.textContent = `Mostrar más (${keys.length - cursor} restantes)`;
        children.append(more);
      }
    }
    more.addEventListener("click", appendPage);
    appendPage();
  });
  return details;
}
