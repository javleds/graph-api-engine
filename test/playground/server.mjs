import { spawn } from 'node:child_process';
import { randomBytes, createHash } from 'node:crypto';
import { mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const directory = await mkdtemp(join(tmpdir(), 'api-engine-playground-'));
let service;
let closing = false;
const provider = createServer((request, response) => {
  response.setHeader('Content-Type', 'application/json');
  response.end('{"data":[{"id":9007199254740993,"name":"Fixture product"}]}');
});
async function close(code = 0) {
  if (closing) return;
  closing = true;
  if (service && service.exitCode === null && service.signalCode === null) {
    service.kill('SIGTERM');
    const timer = setTimeout(() => service.kill('SIGKILL'), 8000);
    await new Promise(resolve => service.once('exit', resolve));
    clearTimeout(timer);
  }
  await new Promise(resolve => provider.close(resolve));
  await rm(directory, { recursive: true, force: true });
  process.exit(code);
}
process.on('SIGTERM', () => close());
process.on('SIGINT', () => close());
try {
  await new Promise((resolve, reject) => {
    provider.once('error', reject);
    provider.listen(18781, '127.0.0.1', resolve);
  });
  const config = JSON.parse(await readFile('config/config.example.json', 'utf8'));
  config.http.address = '127.0.0.1:18780';
  config.http.publicBaseUrl = 'http://127.0.0.1:18780';
  config.storage.path = join(directory, 'engine.db');
  config.storage.minimumFreeDiskBytes = 1;
  config.cache.path = join(directory, 'cache.db');
  config.security.identityKeyFile = join(directory, 'identity.key');
  config.security.encryption.keyFiles = { test: join(directory, 'encryption.key') };
  config.security.encryption.activeKeyId = 'test';
  config.security.principalsFile = join(directory, 'principals.json');
  config.security.egress.upstreamAllowedOrigins = ['http://127.0.0.1:18781'];
  config.security.egress.webhookAllowedOrigins = ['http://127.0.0.1:18781'];
  config.security.egress.allowedPrivateCIDRs = ['127.0.0.0/8'];
  config.security.egress.allowHTTPForDevelopment = true;
  config.fetch.attemptTimeout = '1s';
  config.throttle.safetyMargin = '1ms';
  config.shutdown.gracePeriod = '3s';
  await writeFile(config.security.identityKeyFile, randomBytes(32), { mode: 0o600 });
  await writeFile(config.security.encryption.keyFiles.test, randomBytes(32), { mode: 0o600 });
  await writeFile(config.security.principalsFile, JSON.stringify({ version: 1, principals: [{
    ownerId: 'browser-test', enabled: true, permissions: ['executions:create', 'executions:read'],
    tokenSha256: createHash('sha256').update(randomBytes(32)).digest('hex'),
  }] }), { mode: 0o600 });
  await writeFile(join(directory, 'config.json'), JSON.stringify(config), { mode: 0o600 });
  const binary = join(directory, 'api-engine');
  const build = spawn('go', ['build', '-o', binary, './cmd/api-engine'], { stdio: 'inherit' });
  if (await new Promise(resolve => build.once('exit', resolve)) !== 0) throw new Error('Go build failed');
  service = spawn(binary, ['serve', '--config', join(directory, 'config.json')], { stdio: 'inherit' });
  service.once('exit', code => { if (!closing) close(code || 1); });
} catch (error) {
  console.error(error);
  await close(1);
}
