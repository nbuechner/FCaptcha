# Browser tests

Playwright tests that exercise the client widget in real browsers — covering the parallel PoW solver path that the Node-based suite (`test/test-detection.js`) cannot reach.

## Run

```bash
# 1. Start any FCaptcha server on port 3000 (Node, Python, or Go)
cd ../../server-node && npm start
# or: cd ../../server-go && go run .

# 2. In another terminal, install and run
cd test/browser
npm install
npm run install-browsers   # one-time: downloads Chromium
npm test
```

## What's covered

- `FCaptcha.execute()` solves a PoW in-browser and produces a token the server accepts
- Multi-threaded fan-out: spawns ~`hardwareConcurrency / 2` workers
- Single-thread fallback when `hardwareConcurrency = 1`
- Workers are terminated after solve

## Why a separate package

Playwright pulls a chunky browser binary; isolating its dependency tree from the production server packages keeps `server-node/` lean.
