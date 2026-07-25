# AGC web UI

The production SPA is built with Vite, React, and TypeScript. Its committed
`dist/` directory is embedded into the AGC Go binary.

## Development

Start the AGC server on port 8080, then run:

```sh
npm install
npm run dev
```

Vite serves the SPA locally and proxies `/api` requests to
`http://localhost:8080`.

Create the production bundle with:

```sh
npm run build
```
