import { test, expect } from '@playwright/test';

async function login(page) {
  await page.goto('/login');
  await page.getByLabel('Usuario').fill('browser-test');
  await page.getByLabel('Contraseña').fill('browser-test-password');
  await page.getByRole('button', { name: 'Entrar al playground' }).click();
  await expect(page).toHaveURL(/\/playground$/);
}

test('login rejects invalid credentials and protects the playground', async ({ page, context }) => {
  await context.clearCookies();
  await page.goto('/playground');
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel('Usuario').fill('browser-test');
  await page.getByLabel('Contraseña').fill('wrong');
  await page.getByRole('button', { name: 'Entrar al playground' }).click();
  await expect(page.getByRole('alert')).toContainText('Credenciales incorrectas');
  await page.getByLabel('Contraseña').fill('browser-test-password');
  await page.getByRole('button', { name: 'Entrar al playground' }).click();
  await expect(page).toHaveURL(/\/playground$/);
  await expect(page.locator('#request')).toBeVisible();
});

test('documentation and Pokémon example are available after login', async ({ page }) => {
	await login(page);
  await page.goto('/docs');
  await expect(page.getByRole('heading', { name: 'Opera y consume API Engine.' })).toBeVisible();
  await page.getByLabel('Buscar propiedad de configuración').fill('authenticationEnabled');
  await expect(page.locator('.configuration-section')).toContainText('security.authenticationEnabled');
  await expect(page.locator('.configuration-section')).not.toContainText('PLAYGROUND_PASSWORD');
  await page.getByRole('tab', { name: 'Consumo de grafos' }).click();
  await page.getByLabel('Buscar parámetro de request').fill('children');
  await expect(page.locator('.request-table')).toContainText('calls[].children');
  await page.goto('/examples');
  await expect(page.locator('.examples-grid article')).toHaveCount(3);
  const examplesFitTheirCards = await page.locator('.examples-grid article').evaluateAll((cards) => cards.every((card) => {
    const code = card.querySelector('pre');
    return code && code.getBoundingClientRect().right <= card.getBoundingClientRect().right;
  }));
  expect(examplesFitTheirCards).toBe(true);
  await page.getByRole('button', { name: 'Replay V1 →' }).click();
  await expect(page).toHaveURL(/\/playground\?example=pokemon-v1-webhook$/);
  await expect(page.locator('#request')).toHaveValue(/pokeapi\.co/);
  await expect(page.locator('#request')).toHaveValue(/pokemon_entries/);
});

test('replaying an example presents a positive loading state', async ({ page }) => {
  await login(page);
  await page.goto('/examples');
  await page.getByRole('button', { name: 'Replay V1 →' }).click();
  const message = page.locator('#message');
  await expect(message).toHaveAttribute('role', 'status');
  await expect(message).toHaveAttribute('data-kind', 'info');
  await expect(message).toHaveCSS('background-color', 'rgb(9, 40, 58)');
  await expect(message).toContainText('Ejemplo V1 con webhook local cargado');
});

test('results documentation compares API and webhook behavior with an accessible version selector', async ({ page }) => {
  let executionsCreated = 0;
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/executions$/.test(new URL(request.url()).pathname)) executionsCreated++;
  });
  await page.goto('/docs');
  const tab = page.getByRole('tab', { name: 'Obtención de resultados' });
  await tab.click();
  await expect(tab).toHaveAttribute('aria-selected', 'true');
  const panel = page.getByRole('tabpanel', { name: 'Obtención de resultados' });
  const v1 = panel.getByRole('radio', { name: 'V1 · Resultado final' });
  const v2 = panel.getByRole('radio', { name: 'V2 · Cadenas completas' });
  await expect(v1).toBeChecked();
  await expect(panel.getByRole('heading', { name: 'V1 · El grafo en una respuesta final' })).toBeVisible();
  await expect(panel).toContainText('/v1/executions/{id}/result');
  await expect(panel).toContainText('resultOmittedReason: "size_limit"');
  await expect(panel).toContainText('RESULT_TOO_LARGE');
  await expect(panel).not.toContainText('WEBHOOK_PAYLOAD_TOO_LARGE');
  await v2.check();
  await expect(panel.getByRole('heading', { name: 'V2 · Cada cadena completa, sin esperar al resto' })).toBeVisible();
  await expect(panel).toContainText('/v2/executions/{id}/results?after=0&limit=100');
  await expect(panel).toContainText('/results/{nodeId}');
  await expect(panel).toContainText('node.finished');
  await expect(panel).toContainText('RESULT_VALUE_TOO_LARGE');
  await expect(panel).toContainText('WEBHOOK_PAYLOAD_TOO_LARGE');
  await expect(panel).toContainText('Aumentar ese valor no amplía el máximo del chunk');
  await expect(panel).toContainText('duplicados y entregas fuera de orden');
  await expect(panel).toContainText('422');
  await expect(panel).not.toContainText('resultOmittedReason');
  await v2.focus();
  await page.keyboard.press('ArrowLeft');
  await expect(v1).toBeChecked();
  await expect(panel.getByRole('heading', { name: 'V1 · El grafo en una respuesta final' })).toBeVisible();
  await tab.focus();
  await page.keyboard.press('Home');
  await expect(page.getByRole('tab', { name: 'Servicio y arranque' })).toBeFocused();
  await expect(page.getByLabel('Buscar propiedad de configuración')).toBeVisible();
  await page.keyboard.press('End');
  await expect(tab).toBeFocused();
  await expect(panel).toBeVisible();
  expect(executionsCreated).toBe(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
