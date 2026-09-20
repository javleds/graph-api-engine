import { t } from "./i18n.js";

export function initializeInputEditor(
  validationURL = () => "/playground/api/validate",
) {
  const input = document.getElementById("request");
  const lines = document.getElementById("request-lines");
  const button = document.getElementById("validate-json");
  const message = document.getElementById("validation-message");
  let generation = 0;
  let controller;

  function show(text, kind) {
    message.textContent = text;
    message.dataset.kind = kind;
    message.hidden = false;
  }
  function update() {
    generation++;
    controller?.abort();
    button.disabled = false;
    message.hidden = true;
    message.textContent = "";
    input.removeAttribute("aria-invalid");
    const count = input.value.split("\n").length;
    lines.textContent = Array.from(
      { length: count },
      (_, index) => index + 1,
    ).join("\n");
    lines.parentElement.style.width = `${Math.max(5, String(count).length + 2)}ch`;
    lines.scrollTop = input.scrollTop;
  }
  input.addEventListener("input", update);
  input.addEventListener("scroll", () => {
    lines.scrollTop = input.scrollTop;
  });
  window.addEventListener("pagehide", () => controller?.abort());
  button.addEventListener("click", async () => {
    const current = ++generation;
    controller?.abort();
    controller = new AbortController();
    const raw = input.value;
    try {
      JSON.parse(raw);
    } catch (error) {
      input.setAttribute("aria-invalid", "true");
      show(syntaxMessage(raw, error), "error");
      return;
    }
    button.disabled = true;
    show(t("validating"), "pending");
    try {
      const response = await fetch(validationURL(), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: raw,
        cache: "no-store",
        signal: controller.signal,
      });
      const text = await response.text();
      if (current !== generation) return;
      let body;
      try {
        body = JSON.parse(text);
      } catch {
        throw new Error(
          `El validador devolvió una respuesta no JSON (HTTP ${response.status}).`,
        );
      }
      if (response.ok && body.valid === true) {
        input.removeAttribute("aria-invalid");
        show(
          "JSON válido: cumple el schema y las reglas de la definición. No se creó ningún job.",
          "success",
        );
      } else if ([400, 413, 415, 422].includes(response.status)) {
        const details = body.error?.details ?? [];
        const location = details
          .map((detail) => `${detail.path || "/"}: ${detail.message}`)
          .join(" · ");
        const code = body.error?.code ?? `HTTP ${response.status}`;
        input.setAttribute("aria-invalid", "true");
        show(
          `JSON no válido para la API (${code}). ${location || body.error?.message || "Revisa la estructura del request."}`,
          "error",
        );
      } else {
        throw new Error(
          `No se pudo validar (HTTP ${response.status}). Intenta nuevamente; no se creó ningún job.`,
        );
      }
    } catch (error) {
      if (current === generation && error.name !== "AbortError") {
        show(`No se pudo completar la validación. ${error.message}`, "error");
      }
    } finally {
      if (current === generation) button.disabled = false;
    }
  });
  update();
}

function syntaxMessage(raw, error) {
  const position = error.message.match(/position (\d+)/i);
  if (position) {
    const before = raw.slice(0, Number(position[1]));
    const line = before.split("\n").length;
    const column = before.length - before.lastIndexOf("\n");
    return `JSON inválido · línea ${line}, columna ${column}. ${error.message}`;
  }
  return `JSON inválido. ${error.message}`;
}
