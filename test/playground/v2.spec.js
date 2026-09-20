import { test, expect } from "@playwright/test";

const id = "22222222-2222-4222-8222-222222222222";
const endpoint = `/playground/api/v2/executions/${id}`;
const requestBody = JSON.stringify({
  definition: {
    version: 1,
    api: {
      baseUrl: "http://127.0.0.1:18781",
      limits: { requests: 100, per: "1s" },
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
});

async function prepare(page) {
  await page.goto("/playground");
  await page.locator("#execution-version").selectOption("2");
  await page.locator("#request").fill(requestBody);
}

test("complete result stays mounted and readable during later arrivals", async ({ page }) => {
  let sequence = "1";
  let reads = 0;
  await page.route("**/playground/api/v2/executions", route => route.fulfill({ status: 202, json: { id, status: "pending" } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: { id, status: "running", v2: { resultsEnabled: true, lastResultSequence: sequence, resultCount: Number(sequence) } } }));
  await page.route(`**${endpoint}/results?*`, route => {
    reads++;
    const through = new URL(route.request().url()).searchParams.get("through");
    return route.fulfill({ json: { results: [{ eventId: "event1", nodeId: "bulbasaur", root: "pokemon", index: 0, sequence: "1", status: "completed" }], nextCursor: "1", lastSequence: through, hasMore: false, terminal: false } });
  });
  await page.route(`**${endpoint}/results/bulbasaur`, route => route.fulfill({ contentType: "application/json", body: '{"nodeId":"bulbasaur","data":{"id":9007199254740993,"species":{"name":"bulbasaur"}},"complete":true}' }));
  await prepare(page);
  await page.locator("#execute").click();
  const row = page.locator(".v2-result-list button").first();
  await row.click();
  await expect(page.locator(".v2-result-detail")).toContainText("Cadena terminada");
  await page.getByText("JSON original", { exact: true }).click();
  await expect(page.getByLabel("Resultado JSON original (solo lectura)")).toHaveValue(/9007199254740993/);
  await row.evaluate(node => { node.dataset.stable = "yes"; node.focus(); });
  sequence = "3";
  await expect(page.getByRole("button", { name: "Hay 2 resultados nuevos. Ver resultados nuevos" })).toBeVisible();
  await expect(row).toHaveAttribute("data-stable", "yes");
  await expect(row).toBeFocused();
  await expect(page.getByLabel("Resultado JSON original (solo lectura)")).toBeVisible();
  expect(reads).toBe(1);
  await page.getByRole("button", { name: "Hay 2 resultados nuevos. Ver resultados nuevos" }).click();
  await expect.poll(() => reads).toBe(2);
});

test("large completed result offers bounded branch exploration and download", async ({ page }) => {
  await page.route("**/playground/api/v2/executions", route => route.fulfill({ status: 202, json: { id, status: "pending" } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: { id, status: "completed", v2: { resultsEnabled: true, lastResultSequence: "1", resultCount: 1 } } }));
  await page.route(`**${endpoint}/results?*`, route => route.fulfill({ json: { results: [{ nodeId: "large", root: "pokemon", status: "completed" }], nextCursor: "1", hasMore: false, terminal: true } }));
  await page.route(`**${endpoint}/results/large`, route => route.fulfill({ json: { data: "x".repeat(1024 * 1024), complete: true } }));
  await prepare(page);
  await page.locator("#execute").click();
  await page.locator(".v2-result-list button").click();
  await expect(page.getByRole("button", { name: "Explorar ramas del resultado" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Descargar resultado JSON" })).toHaveAttribute("href", `${endpoint}/results/large`);
  await expect(page.getByLabel("Resultado JSON original (solo lectura)")).toHaveCount(0);
});

test("terminal execution tracks pending webhooks without replacing its result", async ({ page }) => {
  let delivered = false;
  let resultReads = 0;
  await page.route("**/playground/api/v2/executions", route => route.fulfill({ status: 202, json: { id, status: "pending" } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: { id, status: "completed", v2: { resultsEnabled: true, lastResultSequence: "1", resultCount: 1 }, nodeWebhookSummary: { pending: delivered ? 0 : 1, delivered: delivered ? 1 : 0, failed: 0 } } }));
  await page.route(`**${endpoint}/results?*`, route => {
    resultReads++;
    return route.fulfill({ json: { results: [{ nodeId: "node", root: "pokemon", status: "completed" }], nextCursor: "1", lastSequence: "1", hasMore: false, terminal: true } });
  });
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator("#tracking-state")).toHaveText("Ejecución terminada · notificaciones pendientes");
  const row = page.locator(".v2-result-list button");
  await row.evaluate(node => { node.dataset.stable = "yes"; node.focus(); });
  await expect(page.locator("#stop")).toBeEnabled();
  delivered = true;
  await expect(page.locator("#tracking-state")).toHaveText("Ejecución terminada");
  await expect(page.locator(".v2-results")).toContainText("0 pendientes · 1 entregados · 0 fallidos");
  await expect(row).toHaveAttribute("data-stable", "yes");
  await expect(row).toBeFocused();
  expect(resultReads).toBe(1);
});

test("first results read uses the server snapshot when a node closes after status", async ({ page }) => {
  let sequence = "0";
  const queries = [];
  await page.route("**/playground/api/v2/executions", route => route.fulfill({ status: 202, json: { id, status: "pending" } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: { id, status: "running", v2: { resultsEnabled: true, lastResultSequence: sequence, resultCount: Number(sequence) } } }));
  await page.route(`**${endpoint}/results?*`, route => {
    const query = new URL(route.request().url()).searchParams;
    queries.push(query.get("through"));
    sequence = "1";
    return route.fulfill({ json: { results: [{ nodeId: "node", root: "pokemon", status: "completed" }], nextCursor: "1", lastSequence: "1", hasMore: true, terminal: false } });
  });
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator(".v2-result-list")).toContainText("pokemon");
  await page.getByRole("button", { name: "Resultados siguientes" }).click();
  await expect.poll(() => queries.length).toBe(2);
  expect(queries).toEqual(["0", "1"]);
  await expect(page.getByRole("button", { name: /Ver resultados nuevos/ })).toBeHidden();
});

test("result pagination keeps its cutoff and ignores an obsolete detail response", async ({ page }) => {
  let sequence = "2";
  let firstDetail;
  const queries = [];
  await page.route("**/playground/api/v2/executions", route => route.fulfill({ status: 202, json: { id, status: "pending" } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: { id, status: "running", v2: { resultsEnabled: true, lastResultSequence: sequence, resultCount: Number(sequence) } } }));
  await page.route(`**${endpoint}/results?*`, route => {
    const query = new URL(route.request().url()).searchParams;
    queries.push({ after: query.get("after"), through: query.get("through") });
    const second = query.get("after") === "1";
    return route.fulfill({ json: { results: [{ nodeId: second ? "second" : "first", root: second ? "second" : "first", status: "completed" }], nextCursor: second ? "2" : "1", hasMore: !second, terminal: false } });
  });
  await page.route(`**${endpoint}/results/first`, route => { firstDetail = route; });
  await page.route(`**${endpoint}/results/second`, route => route.fulfill({ json: { data: "second result", complete: true } }));
  await prepare(page);
  await page.locator("#execute").click();
  await page.locator(".v2-result-list button").click();
  await expect.poll(() => Boolean(firstDetail)).toBe(true);
  sequence = "3";
  await expect(page.getByRole("button", { name: "Hay 1 resultados nuevos. Ver resultados nuevos" })).toBeVisible();
  await page.getByRole("button", { name: "Resultados siguientes" }).click();
  await expect(page.locator(".v2-result-list")).toContainText("second");
  await page.locator(".v2-result-list button").click();
  await expect(page.locator(".v2-result-detail")).toContainText("second result");
  await firstDetail.fulfill({ json: { data: "obsolete first result", complete: true } }).catch(() => {});
  await expect(page.locator(".v2-result-detail")).not.toContainText("obsolete first result");
  await page.getByRole("button", { name: "Resultados anteriores" }).click();
  await expect(page.locator(".v2-result-list")).toContainText("first");
  expect(queries).toEqual([{ after: "0", through: "2" }, { after: "1", through: "2" }, { after: "0", through: "2" }]);
});

test("v2 lazy tree, bounded chunks and exact readonly scalar JSON", async ({
  page,
}) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/playground/api/v2/executions", (route) =>
    route.fulfill({ status: 202, json: { id, status: "pending" } }),
  );
  await page.route(`**${endpoint}`, (route) =>
    route.fulfill({
      json: {
        id,
        status: "completed",
        v2: { publishedBytes: 12345, recoveries: 2 },
      },
    }),
  );
  await page.route(`**${endpoint}/chunks?*`, (route) => {
    const after = Number(
      new URL(route.request().url()).searchParams.get("after"),
    );
    const last = Math.min(after + 100, 250);
    const chunks = Array.from(
      { length: last - after },
      (_, i) =>
        `{"sequence":"${after + i + 1}","type":"data","operation":"value","value":9007199254740993}`,
    ).join(",");
    return route.fulfill({
      contentType: "application/json",
      body: `{"chunks":[${chunks}],"nextCursor":"${last}","hasMore":${last < 250},"terminal":true,"lastSequence":"250"}`,
    });
  });
  let childReads = 0;
  await page.route(`**${endpoint}/tree?*`, (route) => {
    const child = new URL(route.request().url()).searchParams.get("parentId");
    if (child) childReads++;
    return route.fulfill({
      json: {
        nodes: child
          ? [
              {
                nodeId: "child",
                key: "<img src=x onerror=alert(1)>",
                kind: "value",
                value: null,
                status: "completed",
              },
            ]
          : [
              {
                nodeId: "root",
                key: "items",
                kind: "array",
                status: "completed",
              },
            ],
        nextCursor: "1",
        hasMore: false,
        terminal: true,
        lastSequence: "250",
      },
    });
  });
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator("#tracking-state")).toHaveText(
    "Ejecución terminada",
  );
  expect(childReads).toBe(0);
  await page
    .getByRole("button", { name: "items: array · completed", exact: true })
    .click();
  await expect(page.locator(".v2-nodes")).toContainText(
    "<img src=x onerror=alert(1)>",
  );
  expect(await page.locator(".v2-nodes img").count()).toBe(0);
  await page.getByRole("button", { name: "Chunks", exact: true }).click();
  await expect(page.locator(".v2-chunks button")).toHaveCount(200);
  await page.locator(".v2-chunks button").last().click();
  await expect(
    page.getByLabel("Fragmento JSON original (solo lectura)"),
  ).toHaveValue(
    '{"sequence":"250","type":"data","operation":"value","value":9007199254740993}',
  );
  await expect(
    page.getByLabel("Fragmento JSON original (solo lectura)"),
  ).toHaveAttribute("readonly", "");
  await expect(
    page.getByRole("link", { name: "Descargar NDJSON" }),
  ).toHaveAttribute("href", `${endpoint}/export`);
  await expect(page.locator("#v2-output")).toContainText("2 recuperaciones");
  expect(errors).toEqual([]);
});

test("v2 validation follows selected mode without creating jobs", async ({
  page,
}) => {
  let validations = 0;
  await page.route("**/playground/api/v2/validate", (route) => {
    validations++;
    return route.fulfill({ json: { valid: true } });
  });
  await prepare(page);
  await page.locator("#validate-json").click();
  await expect(page.locator("#validation-message")).toContainText(
    "JSON válido",
  );
  expect(validations).toBe(1);
});

test("real v2 service publishes complete results with diagnostic chunks", async ({
  page,
}) => {
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator("#tracking-state")).toHaveText(
    "Ejecución terminada",
    { timeout: 15000 },
  );
  await expect(page.locator("#status")).toContainText("Completado");
  await expect(page.locator(".v2-result-list")).toContainText("items");
  await page.getByText("Diagnóstico: árbol incremental y chunks", { exact: true }).click();
  await page.getByRole("button", { name: "Chunks", exact: true }).click();
  await expect(page.locator(".v2-chunks button").first()).toBeVisible();
});

test("v2 stops tracking and resumes from confirmed cursor without another POST", async ({
  page,
}) => {
  let posts = 0;
  let complete = false;
  const cursors = [];
  await page.route("**/playground/api/v2/executions", (route) => {
    posts++;
    return route.fulfill({ status: 202, json: { id, status: "pending" } });
  });
  await page.route(`**${endpoint}`, (route) =>
    route.fulfill({ json: { id, status: complete ? "completed" : "running" } }),
  );
  await page.route(`**${endpoint}/chunks?*`, (route) => {
    const after = new URL(route.request().url()).searchParams.get("after");
    cursors.push(after);
    return route.fulfill({
      json: {
        chunks:
          after === "0"
            ? [{ sequence: "1", type: "data", operation: "declare" }]
            : [],
        nextCursor: "1",
        lastSequence: "1",
        terminal: complete,
        hasMore: false,
      },
    });
  });
  await page.route(`**${endpoint}/tree?*`, (route) =>
    route.fulfill({
      json: {
        nodes: [],
        nextCursor: "0",
        hasMore: false,
        terminal: complete,
        lastSequence: "1",
      },
    }),
  );
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator("#v2-output")).toContainText("Cursor 1");
  await page.locator("#stop").click();
  await expect(page.locator("#tracking-state")).toContainText(
    "Seguimiento detenido",
  );
  complete = true;
  await page.locator("#resume").click();
  await expect(page.locator("#tracking-state")).toHaveText(
    "Ejecución terminada",
  );
  expect(posts).toBe(1);
  expect(cursors.at(-1)).toBe("1");
});

test("v2 tree preserves scalar number types and empty property names", async ({
  page,
}) => {
  await page.route("**/playground/api/v2/executions", (route) =>
    route.fulfill({ status: 202, json: { id, status: "pending" } }),
  );
  await page.route(`**${endpoint}`, (route) =>
    route.fulfill({
      json: { id, status: "completed", v2: { lastSequence: "1" } },
    }),
  );
  await page.route(`**${endpoint}/chunks?*`, (route) =>
    route.fulfill({
      json: {
        chunks: [],
        nextCursor: "1",
        lastSequence: "1",
        hasMore: false,
        terminal: true,
      },
    }),
  );
  await page.route(`**${endpoint}/tree?*`, (route) =>
    route.fulfill({
      contentType: "application/json",
      body: '{"nodes":[{"nodeId":"large","key":"id","kind":"value","value":9007199254740993,"status":"completed"},{"nodeId":"empty","path":"/root/","kind":"value","value":false,"status":"completed"}],"nextCursor":"2","hasMore":false,"terminal":true,"lastSequence":"2"}',
    }),
  );
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator(".v2-nodes")).toContainText(
    "id: 9007199254740993 · completed",
  );
  await expect(page.locator(".v2-nodes")).toContainText(
    '"": false · completed',
  );
});
