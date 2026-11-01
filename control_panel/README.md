# Control panel

The Tasks control panel: a static, build-free page (Alpine.js, Tailwind Play,
Lucide, xterm.js, js-yaml, all vendored in `vendor/`, see `vendor/VERSIONS.md`).
It lists Tasks, starts/stops/deletes them, opens terminals into a Task over
WebSocket, and reads and posts messages. agc embeds this folder and serves it
at `/panel/`; every API call is same-origin under `/api/...`.

Tasks have a `title` (free text, shown everywhere) and a `name` (the slug the
server derives from the title: VM name, `#name` channel, URL segment). The
create modal previews the slug with `slugify` in `app.js`, which mirrors the
server rule.

The create modal never shows YAML unless asked: the spec is edited as cards
(Machine, Code, Secrets, Env, Volumes, Channels) of chips, slots and inputs,
following the rtsdesign rules (one Lucide icon per concept, explanations in
`title=` tooltips, no help text). `specToForm` / `formToSpec` in `app.js`
convert between the spec and the form; empty fields are left out of the
spec. The `YAML` toggle in the modal header swaps the cards for the dumped
spec as editable text and parses it back on the way out. Secrets chips come
from `GET /api/secrets` (`{"secrets":[{"name":..., "hosts":[...]}]}`); a
secret with no known hosts asks for them and is sent as `NAME@host1,host2`.

Saved specs (`/api/specs`) are named YAML starting points: pick one from the
"Start from" tiles, save the current form with "Save spec", or save a task's
spec from its Spec tab with "Save as spec…".

Files: `index.html` (markup), `app.js` (all logic), `styles.css` (what Tailwind
cannot express). No npm, no bundler: edit and reload.

Run it without agc against the mock backend (Python 3 stdlib only):

    python3 control_panel/dev/mock_server.py --port 8765
    open http://127.0.0.1:8765/panel/

The mock keeps tasks, terminals and messages in memory and has no WebSocket,
so opening a terminal shows "connecting…" and then the close reason.
