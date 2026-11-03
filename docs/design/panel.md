# The panel's design system

Franco decided it on 2026-11-02 after a test on the Tasks page: the control
panel reads like an instrument, the way the displays in Neon Genesis
Evangelion and Blade Runner do. It is the whole look, not a theme or a
toggle, and it carries no branding from either: the grammar, never the
costume. It sits on top of the rtsdesign method (icons, slots, numbers,
color and shape tell everything; statuses verbatim from the backend;
disabled controls say why in their own label). This file is the reference;
`panel/src/index.css` is where it lives. A panel change follows this file
(rule `panel-follows-the-design-system`).

## Why

The feeling Franco wants is "very dopaminergic to see, like a videogame, but
also clear". Games feel good because things respond: a light flips, a bar
fills, a frame changes color, completion gets a short flourish. agc already
produces those events (a task delivered, an accept, an agent waking, a
watchdog alert, a message), so the look is built to turn each into a
visible moment, and to keep everything else quiet.

## Palette

Color always means something. Nothing is colored for decoration.

| Token | Role | Value |
| --- | --- | --- |
| ground | the page, warm near-black, with a faint orange grid every 24px | `oklch(0.13 0.008 60)` |
| plate | a panel's fill, one step lighter | `oklch(0.16 0.01 60)` |
| orange `--eva` | the system's own voice: frames, tabs, indexes, acts, the current area | `oklch(0.74 0.19 48)` |
| orange dim | one-pixel frames and rails (40%), tints (12%) | `--eva-dim`, `--eva-faint` |
| text | pale warm white for titles and primary text | `oklch(0.92 0.02 80)` |
| muted | warm grey for secondary text and settled states | `oklch(0.62 0.03 70)` |
| green `--eva-green` | alive: running, online | `oklch(0.8 0.2 150)` |
| yellow `--eva-yellow` | waiting on a human: review | `oklch(0.86 0.18 92)` |
| red `--eva-red` | dead or needs a hand now: crashed, failed, destructive | `oklch(0.62 0.25 27)` |
| cyan `--eva-cyan` | you: the page you are on, selection, focus | `oklch(0.8 0.14 200)` |
| ink `--eva-ink` | text on a filled block | `oklch(0.1 0 0)` |

If everything glows, nothing glows: filled blocks (black text on color) are
for the alive, the dead, the current and the acts. Everything settled is
outlined and dim. Every text color meets AA on its ground.

## Type

- Labels, titles, headers, area and page names, the status word: the
  condensed face (`Avenir Next Condensed`, then `Helvetica Neue`, `Arial
  Narrow`, `Roboto Condensed`), uppercase, weight 600, letter spacing
  0.08em (0.18em on the page title).
- Identifiers, people, models, ids, ages, counts: the monospace, lowercase
  as sent. Names are case sensitive and meant to be read, not shouted.
- One large thing per row: the title. The meta line is a step smaller and
  muted.

## Shape

- Radius zero everywhere. Corners are cut, not rounded: plates and filled
  tabs are chamfered at the top right and bottom left (`--cut`, 12px on a
  plate, 6 to 10px on a tab or a button), drawn with `clip-path` and a
  one-pixel frame that follows the cut (the element's background is the
  frame color; a pseudo-element one pixel inside is the fill).
- A list is a column of plates, each with a two-digit index. A plate's left
  edge carries its status as a tab fused into the frame, top to bottom.
  A plate's frame is its status color (Franco, 2026-11-03: "I want to
  change the outline so it is the same as the status."): the color of its
  tab, steady even when the tab blinks. A row waiting on you says so with
  its orange mark and acts, not with its frame.
- Frames are one pixel. Rulers (a line with a tick every 24px, or every
  12px for a finer one) close the top bar and a list's header strip.
- The page title and the current area header are filled orange tabs. In
  the top bar the area comes first as an outlined tab (one-pixel frame
  following a cut at the top right and bottom left) linking to the area,
  then a dim chevron, then the title tab; an identifier in the title is
  monospace, lowercase, a step smaller. Pages
  hang off a rail under their area header; the page you are on is cyan with
  a cyan left bar.
- Acts (accept, answer, restart) are filled orange blocks at the far right
  of a row, so a row ends with what it asks of you.

## Lights

No LED: the status word is the light. Franco's spec, 2026-11-02 (the second
round, replacing the LED spec of the same day): "Only things that need my
attention should be blinking. Get rid of the led; it's the whole REVIEW
that would blink. If it's not running just make the green outline but not
filled." Color is the kind of thing (rtsdesign rule 6: green work, yellow a
person's turn, blue a machine in transition, red broken). So, in a plate's
status tab: running filled green, steady; starting, stopping and creating
filled blue, steady; review yellow, the whole tab blinking between filled
and outlined; crashed the same in red; failed a steady red outline; done a
green outline, not filled; open and any unknown word a dim green outline
with muted text. The plate's frame takes the tab's color (running and
done green, transition blue, review yellow, crashed, failed and error red,
open and unknown the dim green) and never blinks. Canceled is not a backend status yet; failed is the
steady red until it is. Nothing else on the screen blinks: not the counts
in the header, not the pill in a rail, not a machine in transition. Blink
is hard, once a second (`tab-blink`), never a fade. The four `data-tier`
values (lit, off, attention, idle) stay for sorting. Facts that are not a
status (a criterion met, a check passed) keep a small steady light (`Led`).

## Motion

Only for events. Blinking only for "needs you now" or "alive". Transitions
instant. No scanlines, noise or animation over content. A delivery, an
accept, an agent waking and a watchdog alert each deserve one short visible
moment (a flash of the plate's frame, a count stepping) and nothing more.

## Rules of thumb

1. Statuses are the backend's words, verbatim, never derived in the UI.
2. Nothing is colored for decoration; a color on screen is a state, a frame
   or you.
3. The alive thing is the brightest thing on screen; a done thing never
   outshines a running one; a dead thing never looks like a done one.
4. Data reads in columns: aligned, monospace, lowercase.
5. Every list has indexes, a header strip with counts, and a ruler.
6. No new dependency for a look: system fonts, CSS only, markup kept so
   tests keep their hooks (`data-testid`, `data-tier`, `data-status`,
   `aria-*`).
7. Light mode is gone; `dark` is the look. The old zinc tokens stay only in
   `:root` until the toggle is removed.

## Where the look lives

`panel/src/index.css`: the token block under `.dark`, then the unlayered
Eva rules keyed on the markup's attributes (`data-testid`, `data-tier`,
`aria-label`), so they win over Tailwind utilities without touching
components. Area files under `panel/src/styles/` hold a page's own rules
when it needs more than the shared ones. `panel/CLAUDE.md` carries the
operational notes for agents.
