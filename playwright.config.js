import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './test/playground',
  timeout: 30000,
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:18780', trace: 'retain-on-failure' },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' } },
  ],
  webServer: {
    command: 'node test/playground/server.mjs',
    url: 'http://127.0.0.1:18780/health/ready',
    timeout: 120000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 10000 },
  },
});
