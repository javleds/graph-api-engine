import { test, expect } from '@playwright/test';

const requestBody = JSON.stringify({ definition: { version: 1, api: {
  baseUrl: 'http://127.0.0.1:18781', limits: { requests: 100, per: '1s' },
}, calls: [{ name: 'items', type: 'collection', select: 'data', request: { method: 'GET', path: '/items' } }] } }, null, 2);
const executionID = '11111111-1111-4111-8111-111111111111';
const endpoint = `/playground/api/executions/${executionID}`;
function state(status) {
  return { id: executionID, status, createdAt: new Date().toISOString(), progress: {},
    errors: [], errorSummary: { total: 0, byCode: {} }, roots: [],
    error: status === 'failed' ? { code: 'INTERNAL_WORKER_ERROR', message: 'Fixture failure' } : null,
    links: { self: endpoint, result: `${endpoint}/result` } };
}
async function mockJob(page, status, raw) {
  await page.route('**/playground/api/executions', route => route.fulfill({ status: 202,
    json: { id: executionID, status: 'pending', links: { self: endpoint, result: `${endpoint}/result` } } }));
  await page.route(`**${endpoint}`, route => route.fulfill({ json: state(status) }));
  await page.route(`**${endpoint}/result`, route => route.fulfill({ contentType: 'application/json', body: raw }));
}
async function execute(page) {
  await page.goto('/playground');
  await page.locator('#request').fill(requestBody);
  await page.locator('#execute').click();
}

test('real service completes and preserves exact large integer in raw result', async ({ page }, testInfo) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await execute(page);
  await expect(page.locator('#status')).toContainText('Completado', { timeout: 15000 });
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(/9007199254740993/);
  await expect(page.locator('#response-raw')).toHaveAttribute('readonly', '');
  await expect(page.locator('#response-raw')).toHaveValue(/Fixture product/);
  await page.screenshot({ path: testInfo.outputPath('playground.png'), fullPage: true });
  expect(errors).toEqual([]);
});

test('layout follows viewport and request remains keyboard accessible', async ({ page }, testInfo) => {
  await page.goto('/playground');
  const request = await page.locator('.input-panel').boundingBox();
  const output = await page.locator('.output-panel').boundingBox();
  expect(request).not.toBeNull();
  expect(output).not.toBeNull();
  if (testInfo.project.name === 'mobile') expect(output.y).toBeGreaterThan(request.y + request.height);
  else expect(output.x).toBeGreaterThan(request.x + request.width);
  await page.locator('#request').focus();
  await page.keyboard.type('{');
  await expect(page.locator('#request')).toBeFocused();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('invalid JSON never creates a job', async ({ page }) => {
  let calls = 0;
  await page.route('**/playground/api/executions', route => { calls++; return route.abort(); });
  await page.goto('/playground');
  await page.locator('#request').fill('{ invalid');
  await page.locator('#execute').click();
  await expect(page.locator('#execute')).toBeEnabled();
  expect(calls).toBe(0);
});

test('cache clearing requires confirmation and reports the deleted entries', async ({ page }) => {
  let calls = 0;
  page.on('dialog', dialog => {
    expect(dialog.type()).toBe('confirm');
    expect(dialog.message()).toContain('afecta a todos los usuarios');
    return dialog.accept();
  });
  await page.route('**/playground/api/cache/clear', route => {
    calls++;
    return route.fulfill({ json: { deletedEntries: 7 } });
  });
  await page.goto('/playground');
  await page.getByRole('button', { name: 'Limpiar caché', exact: true }).click();
  await expect(page.locator('#message')).toHaveText('Caché limpiada: 7 entradas eliminadas.');
  await expect(page.getByRole('button', { name: 'Limpiar caché', exact: true })).toBeEnabled();
  expect(calls).toBe(1);
});

test('partial result supports exact raw, safe interactive tree and bounded expansion', async ({ page }) => {
  const raw = '{\n "id":"' + executionID + '", "status":"completed_with_errors", "result":{"large":9007199254740993,"html":"<img src=x onerror=alert(1)>","items":[' + Array.from({ length: 250 }, (_, i) => i).join(',') + ']},"errors":[{"code":"UPSTREAM_HTTP_ERROR"}],"errorSummary":{"total":1},"roots":[]\n}';
  await mockJob(page, 'completed_with_errors', raw);
  await execute(page);
  await expect(page.locator('#status')).toContainText('Completado con errores');
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(raw);
  await page.locator('#mode-tree').click();
  await expect(page.locator('#response-tree')).toBeVisible();
  // Open root and result only; arrays remain lazy until explicitly requested.
  for (let i = 0; i < 4; i++) {
    const closed = page.locator('#response-tree details:not([open]) > summary').first();
    if (!await closed.count()) break;
    await closed.click();
  }
  await expect(page.locator('#response-tree')).toContainText('9007199254740993');
  await expect(page.locator('#response-tree img')).toHaveCount(0);
  const more = page.getByRole('button', { name: /Mostrar más/ });
  await expect(more.first()).toBeVisible();
  expect(await page.locator('#response-tree *').count()).toBeLessThan(1600);
  await more.first().click();
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(raw);
});

test('failed execution shows terminal status error without requesting result', async ({ page }) => {
  let results = 0;
  await mockJob(page, 'failed', '{}');
  await page.route(`**${endpoint}/result`, route => { results++; return route.fulfill({ status: 409 }); });
  await execute(page);
  await expect(page.locator('#status')).toContainText('Fallido');
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(/INTERNAL_WORKER_ERROR/);
  expect(results).toBe(0);
});

test('HTTP non-JSON errors remain visible and POST is never retried', async ({ page }) => {
  let calls = 0;
  await page.route('**/playground/api/executions', route => {
    calls++;
    return route.fulfill({ status: 503, contentType: 'text/plain', body: 'upstream maintenance fixture' });
  });
  await execute(page);
  await expect(page.locator('body')).toContainText('upstream maintenance fixture');
  await expect(page.locator('#execute')).toBeEnabled();
  expect(calls).toBe(1);
});

test('network loss allows resume by ID without creating another job', async ({ page }) => {
  let posts = 0;
  let healthy = false;
  await page.route('**/playground/api/executions', route => {
    posts++;
    return route.fulfill({ status: 202, json: { id: executionID, status: 'pending' } });
  });
  await page.route(`**${endpoint}`, route => healthy ? route.fulfill({ json: state('completed') }) : route.abort());
  await page.route(`**${endpoint}/result`, route => route.fulfill({ json: { result: 'resumed' } }));
  await execute(page);
  await expect(page.locator('#tracking-state')).toHaveText('Seguimiento pausado');
  healthy = true;
  await page.locator('#execution-id').fill(executionID);
  await page.locator('#resume').click();
  await expect(page.locator('#status')).toContainText('Completado');
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(/resumed/);
  expect(posts).toBe(1);
});

test('stopping polling keeps job resumable and ignores an old pending response', async ({ page }) => {
  let release;
  let requested;
  const waiting = new Promise(resolve => { requested = resolve; });
  await page.route('**/playground/api/executions', route => route.fulfill({ status: 202, json: { id: executionID, status: 'pending' } }));
  await page.route(`**${endpoint}`, async route => {
    requested();
    await new Promise(resolve => { release = resolve; });
    await route.fulfill({ json: state('completed') }).catch(() => {});
  });
  await execute(page);
  await waiting;
  await page.locator('#stop').click();
  await expect(page.locator('#execute')).toBeEnabled();
  await page.locator('#clear').click();
  release();
  await expect(page.locator('#status')).not.toContainText('Completado');
});

test('copy uses exact raw text and falls back to selecting readonly output', async ({ page }) => {
  const raw = '{ "id": 9007199254740993, "data": [1, 2] }\n';
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: {
      writeText: async value => { window.copiedText = value; },
    } });
  });
  await mockJob(page, 'completed', raw);
  await execute(page);
  await expect(page.locator('#copy')).toBeEnabled();
  await page.locator('#copy').click();
  expect(await page.evaluate(() => window.copiedText)).toBe(raw);
  await page.evaluate(() => { navigator.clipboard.writeText = async () => { throw new Error('denied'); }; });
  await page.locator('#copy').click();
  await expect(page.locator('#response-raw')).toBeVisible();
  expect(await page.locator('#response-raw').evaluate(element => element.value.slice(element.selectionStart, element.selectionEnd))).toBe(raw);
});

test('unsupported lossless parser falls back to exact raw', async ({ page }) => {
  await page.addInitScript(() => {
    const parse = JSON.parse;
    JSON.parse = (text, reviver) => parse(text, reviver && function (key, value) { return reviver.call(this, key, value); });
  });
  const raw = '{"number":9007199254740993}';
  await mockJob(page, 'completed', raw);
  await execute(page);
  await expect(page.locator('#response-raw')).toBeVisible();
  await expect(page.locator('#response-raw')).toHaveValue(raw);
  await expect(page.locator('#mode-tree')).toBeDisabled();
});

test('validation debug preserves exact submitted snapshot and idempotency', async ({ page }) => {
  const raw = '{ "definition": { "number": 9007199254740993 } }';
  let received;
  await page.route('**/playground/api/executions', route => {
    received = { body: route.request().postData(), key: route.request().headers()['idempotency-key'] };
    return route.fulfill({ status: 422, headers: { 'X-Request-Id': 'fixture-validation' },
      json: { error: { code: 'INVALID_DEFINITION', message: 'Invalid fixture', details: [{ path: 'definition.number', message: 'Unknown field' }] } } });
  });
  await page.goto('/playground');
  await page.locator('#request').fill(raw);
  await page.locator('#idempotency').fill('browser-key');
  await page.locator('#execute').click();
  await expect(page.locator('#tracking-state')).toHaveText('Seguimiento pausado');
  await page.locator('#request').fill('{}');
  expect(received).toEqual({ body: raw, key: 'browser-key' });
  const requestDebug = page.locator('#debug details').filter({ hasText: 'Request enviado' });
  await requestDebug.locator('summary').click();
  await expect(requestDebug).toContainText(raw);
  await expect(requestDebug).toContainText('browser-key');
  const createDebug = page.locator('#debug details').filter({ hasText: 'Creación' });
  await createDebug.locator('summary').click();
  await expect(createDebug).toContainText('422');
  await expect(createDebug).toContainText('Unknown field');
  await expect(createDebug).toContainText('fixture-validation');
});

test('polling waits for each status response before scheduling the next', async ({ page }) => {
  let requests = 0;
  let release;
  let requested;
  const firstRequest = new Promise(resolve => { requested = resolve; });
  await page.clock.install();
  await page.route('**/playground/api/executions', route => route.fulfill({ status: 202, json: { id: executionID, status: 'pending' } }));
  await page.route(`**${endpoint}`, async route => {
    requests++;
    if (requests === 1) {
      requested();
      await new Promise(resolve => { release = resolve; });
      return route.fulfill({ json: state('running') });
    }
    return route.fulfill({ json: state('completed') });
  });
  await page.route(`**${endpoint}/result`, route => route.fulfill({ json: { result: 'done' } }));
  await execute(page);
  await firstRequest;
  await page.clock.fastForward(3500);
  expect(requests).toBe(1);
  release();
  await expect(page.locator('#status')).toContainText('En ejecución');
  await page.clock.fastForward(1000);
  await expect(page.locator('#status')).toContainText('Completado');
  expect(requests).toBe(2);
});

test('webhook refresh waits for result and preserves the displayed response', async ({ page }) => {
  let posts = 0;
  let statuses = 0;
  let release;
  const raw = '{"result":"keep this exact result"}';
  await page.route('**/playground/api/executions', route => {
    posts++;
    return route.fulfill({ status: 202, json: { id: executionID, status: 'pending' } });
  });
  await page.route(`**${endpoint}`, route => {
    statuses++;
    return route.fulfill({ json: { ...state('completed'), webhook: { status: statuses > 1 ? 'delivered' : 'pending', attempts: statuses - 1 } } });
  });
  await page.route(`**${endpoint}/result`, async route => {
    await new Promise(resolve => { release = resolve; });
    return route.fulfill({ contentType: 'application/json', body: raw });
  });
  await execute(page);
  await expect(page.locator('#refresh-webhook')).toBeDisabled();
  await expect.poll(() => typeof release).toBe('function');
  release();
  await expect(page.locator('#refresh-webhook')).toBeEnabled();
  await page.locator('#mode-raw').click();
  await expect(page.locator('#response-raw')).toHaveValue(raw);
  await page.locator('#refresh-webhook').click();
  await expect(page.locator('#webhook')).toContainText('delivered');
  await expect(page.locator('#response-raw')).toHaveValue(raw);
  expect(posts).toBe(1);
  expect(statuses).toBe(2);
});

test('input validation checks real schema without creating a job and numbers lines', async ({ page }) => {
  let jobs = 0;
  page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/api/executions')) jobs++; });
  await page.goto('/playground');
  await page.locator('#request').fill(requestBody);
  await page.getByRole('button', { name: 'Validar JSON', exact: true }).click();
  await expect(page.locator('#validation-message')).toContainText('JSON válido:');
  await expect(page.locator('#status')).toContainText('Sin ejecución');
  expect(jobs).toBe(0);
  await expect(page.locator('#request-lines')).toHaveText(requestBody.split('\n').map((_, i) => i + 1).join('\n'));
  expect(await page.locator('.line-numbers').evaluate(node => getComputedStyle(node).userSelect)).toBe('none');
  await expect(page.locator('.line-numbers')).toHaveAttribute('aria-hidden', 'true');
  await page.locator('#request').fill('{}');
  await expect(page.locator('#validation-message')).toBeHidden();
  await page.locator('#validate-json').click();
  await expect(page.locator('#validation-message')).toContainText('JSON no válido');
  await page.locator('#request').fill('{\n invalid\n}');
  await page.locator('#validate-json').click();
  await expect(page.locator('#validation-message')).toContainText('JSON inválido');
  await page.locator('#request').fill(Array.from({length: 200}, (_, i) => `"line ${i}"`).join('\n'));
  await page.locator('#request').evaluate(node => { node.scrollTop = 400; node.dispatchEvent(new Event('scroll')); });
  const offsets = await page.evaluate(() => [document.getElementById('request').scrollTop, document.getElementById('request-lines').scrollTop]);
  expect(offsets[0]).toBeGreaterThan(0);
  expect(offsets[0]).toBe(offsets[1]);
  await page.locator('#clear').click();
  await expect(page.locator('#request-lines')).toHaveText('1');
  await page.locator('#example').click();
  expect((await page.locator('#request-lines').textContent()).split('\n').length).toBeGreaterThan(1);
  expect(jobs).toBe(0);
});

test('editing input ignores an obsolete validation response', async ({ page }) => {
  let release;
  let requested;
  const pending = new Promise(resolve => { requested = resolve; });
  await page.route('**/playground/api/validate', async route => {
    requested();
    await new Promise(resolve => { release = resolve; });
    await route.fulfill({json: {valid: true}}).catch(() => {});
  });
  await page.goto('/playground');
  await page.locator('#request').fill(requestBody);
  await page.locator('#validate-json').click();
  await pending;
  await page.locator('#request').fill('{}');
  release();
  await expect(page.locator('#validate-json')).toBeEnabled();
  await expect(page.locator('#validation-message')).toBeHidden();
});
