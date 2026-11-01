# Control panel

The agc Tasks control panel: a React + Vite app that agc embeds and serves
at `/panel/`. It lists Tasks, starts and stops them, opens terminals into
them, and reads and posts messages.

## Develop

    cd panel
    npm ci
    npm run mock     # fake API on :8790
    npm run dev      # http://localhost:5173/panel/

To develop against a real agc instead of the mock:

    VITE_API_TARGET=http://127.0.0.1:1202 npm run dev

## Check and build

    npx playwright install chromium   # once
    npm run check                     # types drift, tsc, eslint, Playwright
    make panel                        # from the repo root: builds panel/dist

`make build` and `make install` build the panel before `go build`, so the
binary embeds it. Conventions for contributors (and agents) are in
[CLAUDE.md](CLAUDE.md).
