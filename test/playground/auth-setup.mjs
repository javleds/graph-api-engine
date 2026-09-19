import { mkdir, writeFile } from 'node:fs/promises';

export default async function setup() {
  const response = await fetch('http://127.0.0.1:18780/playground/api/session', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'browser-test', password: 'browser-test-password' }),
  });
  if (!response.ok) throw new Error(`playground login failed: ${response.status}`);
  const setCookie = response.headers.get('set-cookie');
  const value = setCookie?.match(/api_engine_playground_session=([^;]+)/)?.[1];
  if (!value) throw new Error('playground session cookie missing');
  await mkdir('test-results', { recursive: true });
  await writeFile('test-results/auth-state.json', JSON.stringify({ cookies: [{ name: 'api_engine_playground_session', value, domain: '127.0.0.1', path: '/', httpOnly: true, secure: false, sameSite: 'Strict', expires: -1 }], origins: [] }));
}
