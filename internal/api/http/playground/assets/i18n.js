const language = document.documentElement.dataset.uiLanguage === "en" ? "en" : "es";
const catalog = {
  es: { validate: "Validar JSON", validating: "Validando sintaxis, schema y referencias…", clear: "Limpiar", execute: "Ejecutar request", copy: "Copiar JSON", examples: "Ejemplos", logout: "Cerrar sesión", debug: "Depuración HTTP", noExchanges: "Aún no hay intercambios HTTP." },
  en: { validate: "Validate JSON", validating: "Validating syntax, schema, and references…", clear: "Clear", execute: "Run request", copy: "Copy JSON", examples: "Examples", logout: "Sign out", debug: "HTTP debugging", noExchanges: "No HTTP exchanges yet." },
};
export const t = (key) => catalog[language][key] ?? key;
export const locale = language === "en" ? "en-US" : "es-MX";
export function translateStatic() {
  if (language !== "en") return;
  const replacements = new Map([["Ejemplos", t("examples")], ["Cerrar sesión", t("logout")], ["Copiar JSON", t("copy")], ["Validar JSON", t("validate")], ["Limpiar", t("clear")], ["Ejecutar request", t("execute")], ["Depuración HTTP", t("debug")], ["Aún no hay intercambios HTTP.", t("noExchanges")] ]);
  document.querySelectorAll("body *:not(script):not(style)").forEach((node) => { if (node.children.length === 0 && replacements.has(node.textContent)) node.textContent = replacements.get(node.textContent); });
}
