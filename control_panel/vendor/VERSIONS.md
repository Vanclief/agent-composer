# Vendored libraries

Pinned copies served to the browser so the panel works offline. Nothing here
is built or run on the server. To upgrade, download the new file from the
same URL with the new version, update this table, and check the panel.

| File | Package | Version | Source URL | sha256 |
| --- | --- | --- | --- | --- |
| `alpinejs/cdn.min.js` | alpinejs | 3.17.5 | https://cdn.jsdelivr.net/npm/alpinejs@3.17.5/dist/cdn.min.js | `b4036e9e9ac22fa6f6ae1ba292e015e1f3e9c85d0d79e73e4c8eb9bb139b522e` |
| `tailwindcss/tailwindcss-play-3.4.17.js` | Tailwind CSS Play CDN (3.x) | 3.4.17 | https://cdn.tailwindcss.com/3.4.17 | `176e894661aa9cdc9a5cba6c720044cbbf7b8bd80d1c9a142a7c24b1b6c50d15` |
| `lucide/lucide.min.js` | lucide | 1.66.0 | https://cdn.jsdelivr.net/npm/lucide@1.66.0/dist/umd/lucide.min.js | `ba9051c3e42e8354970769a9933fc84d0e40078180b972c6772f88f0d8bca95b` |
| `xterm/xterm.js` | @xterm/xterm | 5.5.0 | https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.js | `1f991ac3b4b283ebf96e60ae23a00a52765dd3a2e46fa6fdda9f1aab032f7495` |
| `xterm/xterm.css` | @xterm/xterm | 5.5.0 | https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.css | `ba8e6985669488981ccf40c0cefe3aba80722cb6c92de7ad628b0bd717faf2b6` |
| `xterm/addon-fit.js` | @xterm/addon-fit | 0.10.0 | https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js | `bdaefa370b1bfc42ee88d46fe6072400902a4d4b2d45cd93438dda9b23c97089` |
| `js-yaml/js-yaml.min.js` | js-yaml | 4.1.0 | https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js | `45dc3dd03dc07a06705a2c2989b8c7f709013f04bd5386e3279d4e447f07ebd7` |

Notes:

- Tailwind: the 3.x Play CDN script (`cdn.tailwindcss.com`, which redirects to
  `/3.4.17`) was picked over `@tailwindcss/browser@4` because it keeps the
  `tailwind.config` / `darkMode: 'class'` setup the mockup already used. It
  compiles classes in the browser and logs a "should not be used in
  production" console warning; that is expected for a build-free panel.
- xterm and the fit addon are the 5.x / 0.10 line on purpose (xterm 6 is out).
- js-yaml stays on the long-standing 4.1.0 release; it exposes the `jsyaml`
  global the panel uses.
- Downloaded 2026-11-01.
