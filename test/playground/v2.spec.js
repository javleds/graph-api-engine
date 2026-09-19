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
        `{"sequence":"${after + i + 1}","type":"data","operation":"scalar","value":9007199254740993}`,
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
                kind: "null",
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
    '{"sequence":"250","type":"data","operation":"scalar","value":9007199254740993}',
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

test("real v2 service publishes chunks and exposes a browsable root", async ({
  page,
}) => {
  await prepare(page);
  await page.locator("#execute").click();
  await expect(page.locator("#tracking-state")).toHaveText(
    "Ejecución terminada",
    { timeout: 15000 },
  );
  await expect(page.locator("#status")).toContainText("Completado");
  await expect(page.locator(".v2-nodes")).toContainText("items");
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
