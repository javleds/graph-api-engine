import { test, expect } from "@playwright/test";

const id = "33333333-3333-4333-8333-333333333333";

async function runningJob(page, version) {
  const base =
    version === "2"
      ? "/playground/api/v2/executions"
      : "/playground/api/executions";
  const endpoint = `${base}/${id}`;
  let cancelled = false;
  await page.route(`**${endpoint}`, (route) =>
    route.fulfill({
      json: {
        id,
        status: cancelled ? "failed" : "running",
        ...(cancelled ? { error: { code: "EXECUTION_CANCELLED" } } : {}),
      },
    }),
  );
  await page.route(`**${endpoint}/chunks?*`, (route) =>
    route.fulfill({
      json: {
        chunks: [{ sequence: "1", type: "data", operation: "declare" }],
        nextCursor: "1",
        lastSequence: "1",
        hasMore: false,
        terminal: cancelled,
      },
    }),
  );
  await page.route(`**${endpoint}/tree?*`, (route) =>
    route.fulfill({
      json: {
        nodes: [],
        nextCursor: "0",
        hasMore: false,
        terminal: cancelled,
        lastSequence: "1",
      },
    }),
  );
  await page.goto("/playground");
  await page.locator("#execution-version").selectOption(version);
  await expect(page.locator("#cancel-execution")).toBeDisabled();
  await page.locator("#execution-id").fill(id);
  await page.locator("#resume").click();
  await expect(page.locator("#status")).toContainText("En ejecución");
  if (version === "2")
    await expect(page.locator("#v2-output")).toContainText("Cursor 1");
  return {
    endpoint,
    cancel: () => {
      cancelled = true;
    },
  };
}

for (const version of ["1", "2"]) {
  test(`v${version} abort reaches server after stopping tracking and retains terminal output`, async ({
    page,
  }) => {
    const job = await runningJob(page, version);
    const requests = [];
    await page.route(`**${job.endpoint}/cancel`, async (route) => {
      requests.push(route.request().method());
      job.cancel();
      await route.fulfill({
        json: { id, status: "failed", error: { code: "EXECUTION_CANCELLED" } },
      });
    });
    await page.locator("#stop").click();
    await expect(page.locator("#tracking-state")).toContainText(
      "Seguimiento detenido",
    );
    expect(requests).toEqual([]);
    await expect(page.locator("#cancel-execution")).toBeEnabled();
    await page.locator("#cancel-execution").click();
    await expect(page.locator("#status")).toHaveText("× Cancelada");
    await expect(page.locator("#tracking-state")).toHaveText(
      "Ejecución terminada",
    );
    await expect(page.locator("#cancel-execution")).toBeDisabled();
    expect(requests).toEqual(["POST"]);
    if (version === "2") {
      await expect(page.locator("#v2-output")).toContainText("Cursor 1");
      await page.getByRole("button", { name: "Chunks", exact: true }).click();
      await expect(page.locator(".v2-chunks button")).toHaveCount(1);
    } else {
      await page.locator("#mode-raw").click();
      await expect(page.locator("#response-raw")).toHaveValue(
        /EXECUTION_CANCELLED/,
      );
    }
  });
}

test("failed cancellation permits explicit retry and preserves HTTP diagnostics", async ({
  page,
}) => {
  const job = await runningJob(page, "1");
  let attempts = 0;
  await page.route(`**${job.endpoint}/cancel`, (route) => {
    attempts++;
    if (attempts === 1)
      return route.fulfill({
        status: 503,
        json: { error: { code: "UNAVAILABLE" } },
      });
    job.cancel();
    return route.fulfill({
      json: { id, status: "failed", error: { code: "EXECUTION_CANCELLED" } },
    });
  });
  await page.locator("#cancel-execution").click();
  await expect(page.locator("#message")).toContainText("HTTP 503");
  await expect(page.locator("#status")).toContainText("En ejecución");
  await expect(page.locator("#cancel-execution")).toBeEnabled();
  await expect(
    page.locator('#debug details[data-key="cancel"] summary'),
  ).toContainText("HTTP 503");
  expect(attempts).toBe(1);
  await page.locator("#cancel-execution").click();
  await expect(page.locator("#status")).toHaveText("× Cancelada");
  expect(attempts).toBe(2);
});

test("a late cancellation response cannot overwrite another version", async ({
  page,
}) => {
  const job = await runningJob(page, "1");
  let release;
  const responseGate = new Promise((resolve) => {
    release = resolve;
  });
  let received;
  const requestGate = new Promise((resolve) => {
    received = resolve;
  });
  await page.route(`**${job.endpoint}/cancel`, async (route) => {
    received();
    await responseGate;
    await route
      .fulfill({
        json: { id, status: "failed", error: { code: "EXECUTION_CANCELLED" } },
      })
      .catch(() => {});
  });
  await page.locator("#cancel-execution").click();
  await requestGate;
  await expect(page.locator("#cancel-execution")).toBeDisabled();
  await page.locator("#execution-version").selectOption("2");
  release();
  await expect(page.locator("#status")).toHaveText("○ Sin estado disponible");
  await expect(page.locator("#tracking-state")).toHaveText(
    "Listo para ejecutar",
  );
  await expect(page.locator("#cancel-execution")).toBeDisabled();
  await expect(page.locator('#debug details[data-key="cancel"]')).toHaveCount(
    0,
  );
});
