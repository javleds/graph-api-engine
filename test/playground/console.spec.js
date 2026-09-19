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
  await page.getByLabel('Buscar propiedad de configuración').fill('PLAYGROUND_PASSWORD');
  await expect(page.locator('.doc-layout aside')).toContainText('PLAYGROUND_PASSWORD');
  await page.getByRole('button', { name: 'Consumo de grafos' }).click();
  await page.getByLabel('Buscar parámetro de request').fill('children');
  await expect(page.locator('.doc-layout aside')).toContainText('calls[].children');
  await page.goto('/examples');
  await page.getByRole('button', { name: /Inténtalo en playground/ }).click();
  await expect(page).toHaveURL(/\/playground\?example=pokemon$/);
  await expect(page.locator('#request')).toHaveValue(/pokeapi\.co/);
  await expect(page.locator('#request')).toHaveValue(/pokemon_entries/);
});
