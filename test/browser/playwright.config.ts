import { defineConfig, devices } from '@playwright/test';

// These tests assume an FCaptcha server (Node, Python, or Go) is already
// running on http://localhost:3000. Start one with `npm start` from
// server-node/, or run server-go/server, before invoking `npm test`.
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
