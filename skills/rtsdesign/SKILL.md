---
name: rtsdesign
description: >-
  Design UI the way strategy and factory games do (Factorio, Satisfactory,
  Age of Empires 2): the screen tells you everything through icons, slots,
  numbers, color and shape, so the user never reads a paragraph or stops to
  think. Use when the user asks for an interface that is "visual", "like a
  game", "intuitive", "no YAML/JSON/code in the UI", "don't make me think",
  or invokes rtsdesign. Not a visual theme: no medieval or sci-fi styling.
---

Design the screen so a user can operate it with every sentence removed. Icons, slots, numbers, color and shape carry the meaning; text is a one or two word label at most. If a control needs a sentence to explain it, redesign the control.

## Principles, in priority order

1. **Show the thing, not a description of it.** A resource is an icon and a number (`⛁ 4 GB`), never "memory: 4096 MiB". A list of items is a row of tiles, never a comma-separated string. Raw formats (YAML, JSON, config text) are never the primary view; keep them behind an "advanced" or `</>` toggle for people who want them.
2. **One icon per concept, everywhere.** Pick the icon for CPU, memory, repo, secret, volume, channel once and use it in every list, card, detail and form. The user learns it once. Use the project's existing icon set; never mix sets.
3. **Color codes concepts, saturation codes attention.** Every concept gets one hue to go with its icon (wood is brown, gold is yellow, in every screen), used on the icon, its chips, its card header and its selected state. Everything that is not a concept stays neutral. Saturation is reserved for what needs the eye: the primary action, the selected chip, a live status, an error. Inert things (unselected chips, empty slots, labels) are dim. A screen where everything is the same grey is as unreadable as one where everything is colored.
4. **Slots make absence visible.** An empty slot is a dashed outline with a `+`, like an empty inventory square. The user sees there is a place for something and how to fill it, without a label saying "none yet" or "add one here".
5. **Numbers come from presets.** A quantity is a row of chips (`1 2 4 8`) or a stepper with sane steps, with the current value lit. Free-text numeric inputs are a last resort. Show units on the chip, not in help text, and show them in the unit humans think in (GB, not MiB).
6. **The status word is the light: color by kind, filled or outlined, blinking only for a hand.** No separate dot or LED: the word itself, printed exactly as the backend sent it, carries the state, in one of four tiers. The color says what kind of thing it is: green is the work itself, yellow is a person's turn, blue is a machine in transition, red is broken. Filled means alive right now; an outline means not running. Blinking is reserved for what needs a hand from the person looking, and it is the whole word that blinks, nothing else on the screen ever does. *Lit*: running is filled green, steady; starting and stopping are filled blue, steady; review is yellow and blinks (a person's turn). *Off*: done is a green outline, full strength, never filled, so finished never looks like running; a cleanly stopped sandbox is the same outline. *Attention*: crashed is red and blinks; failed is a steady red outline; red is the only red on the screen, and a dead thing is never green. *Idle*: open, not started yet, is a dim green outline, so open → running → done reads dim outline → filled → outline in the one color. A word the screen does not know is a dim outline with grey text. In a plain pill away from the list, the colors live in the border and the text and nothing blinks; the filled and blinking looks belong to the list's status column.
7. **Toggles are switches, lists of options are chips.** A boolean is a switch that visibly flips. Picking from a known set is a row of toggle chips; selected chips are filled, unselected are outlined. Mutually exclusive choices are a segmented control.
8. **Group with panels, not headings.** Related controls sit in one bordered card with a single icon in its corner, in the card's concept color. Headings are one word. No section descriptions.
9. **Explanations live in tooltips.** Everything a help sentence would say goes into a hover tooltip on the icon. The layout never contains explanatory prose. The exception is the project rule that a disabled control says in its own label why it is disabled.
10. **One obvious primary action.** Each panel or dialog has one big, lit primary button in the same corner every time. Secondary actions are smaller and outlined. Destructive actions are red, physically separated, and confirmed by a second click in place.
11. **Instant visible feedback.** Every click changes something on screen immediately: a chip fills, a slot gets content, a badge count increments, a button shows a pressed or busy state. Nothing happens silently.
12. **Fixed positions.** The same information lives in the same place on every screen, like a minimap always in the corner. Do not reflow panels based on content. The top-level areas of the app are all on screen at once, as groups the user can collapse, never as modes that hide each other: a collapsed group still shows its header, icon and badge, so nothing that needs attention is ever behind a switch.
13. **Lists are rows, not cards.** A list item is a flat row with a hairline separator, never a bordered, colored card: colored borders are for the selected item and for panels that group controls (rule 8). One row has one light at the left (rule 6), the name in normal weight, and everything else as dim text in a fixed order; a chip in a row is for something you click or something lit (a live count, a warning), never for plain facts like a model name or a person. Ten rows of equal chips read as noise; the eye must land on the light, then the name, then nothing else unless it looks.
14. **Attention is personal.** "Needs a hand" is always someone's hand. When a thing waits on the person looking at the screen, its row carries one lit amber mark at the left edge and a short lit chip naming the act (`accept`, `answer`), and the section's badge counts it. When it waits on someone else, show that name in dim text next to the status (`review · @manager`), never lit: the viewer must see at a glance what is theirs and what is not. Never highlight everything that waits on anyone.
15. **Derived facts are shown as results, not rules.** Instead of "the repo is cloned to /home/agent/<name>", show a path chip that updates as the user types: `📁 /home/agent/agent-composer`.

## Method

1. Inventory every piece of data the screen carries and classify each: quantity, boolean, pick-from-set, free text, list of items, path, derived.
2. Assign each a control from the table below and the project's icon for it.
3. Lay the controls out as cards in a fixed grid, most used first, primary action pinned.
4. Delete every sentence. Move what it said into a tooltip or make the control carry it.
5. Test by hiding all text labels (mentally or with CSS): the screen must still be operable. Then restore the one or two word labels.

| Data | Control |
|---|---|
| quantity with sensible values | preset chips or stepper, unit on chip |
| boolean | switch |
| pick one of a known set | segmented control with icons |
| pick many of a known set | toggle chips |
| list of items | tile row, one tile per item, trailing empty slot with `+` |
| free text | input with a leading icon, placeholder shows an example value |
| path or identifier | mono chip |
| derived value | read-only chip that updates live |
| long optional text (script) | collapsed card that expands on click, never shown open by default |
| raw format for experts | `</>` toggle revealing the text, off by default |

## What this is not

Not pixel art, not medieval or sci-fi styling, not sound effects, not skeuomorphism. Use the project's existing palette and components. The game influence is purely in how information is communicated: densely, graphically, at a glance.
