import { spawn } from 'node:child_process';
import { randomBytes, createHash } from 'node:crypto';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
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
  const identityKeyFile = join(directory, 'identity.key');
  const encryptionKeyFile = join(directory, 'encryption.key');
  const principalsFile = join(directory, 'principals.json');
  await writeFile(identityKeyFile, randomBytes(32), { mode: 0o600 });
  await writeFile(encryptionKeyFile, randomBytes(32), { mode: 0o600 });
  await writeFile(principalsFile, JSON.stringify({ version: 1, principals: [{
    ownerId: 'browser-test', enabled: true, permissions: ['executions:create', 'executions:read'],
    tokenSha256: createHash('sha256').update(randomBytes(32)).digest('hex'),
  }] }), { mode: 0o600 });
  const quote = JSON.stringify;
  const config = [
    'http:',
    '  address: ' + quote('127.0.0.1:18780'),
    '  publicBaseUrl: ' + quote('http://127.0.0.1:18780'),
    'storage:',
    '  path: ' + quote(join(directory, 'engine.db')),
    '  minimumFreeDiskBytes: 1',
    'cache:',
    '  path: ' + quote(join(directory, 'cache.db')),
    'fetch:',
    '  attemptTimeout: "1s"',
    'throttle:',
    '  safetyMargin: "1ms"',
    'shutdown:',
    '  gracePeriod: "3s"',
    'security:',
    '  identityKeyFile: ' + quote(identityKeyFile),
    '  principalsFile: ' + quote(principalsFile),
    '  encryption:',
    '    activeKeyId: test',
    '    keyFiles:',
    '      test: ' + quote(encryptionKeyFile),
    '  playground:',
    '    username: browser-test',
    '    password: browser-test-password',
    '  egress:',
    '    upstreamAllowedOrigins: [' + quote('http://127.0.0.1:18781') + ']',
    '    webhookAllowedOrigins: [' + quote('http://127.0.0.1:18781') + ']',
    '    allowedPrivateCIDRs: [127.0.0.0/8]',
    '    allowHTTPForDevelopment: true',
  ].join('\n');
  const configPath = join(directory, 'config.yml');
  await writeFile(configPath, config, { mode: 0o600 });
  const binary = join(directory, 'api-engine');
  const build = spawn('go', ['build', '-o', binary, './cmd/api-engine'], { stdio: 'inherit' });
  if (await new Promise(resolve => build.once('exit', resolve)) !== 0) throw new Error('Go build failed');
  service = spawn(binary, ['serve', '--config', configPath], { stdio: 'inherit' });
  service.once('exit', code => { if (!closing) close(code || 1); });
} catch (error) {
  console.error(error);
  await close(1);
}
