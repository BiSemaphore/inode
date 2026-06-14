---
name: social-post
description: Generate a paper-and-ink carousel HTML post for the inode project, in the voice of a developer talking to other developers — dry wit, real code, opinionated takes, and visual variety (terminal blocks, architecture sketches, stack callouts). Use when the user wants to share what the project is and convince a developer to spend 5 minutes on it. NOT for marketing pitches and NOT for plain philosophy essays.
---

# Social post generator (developer-blog style)

## What this skill is for

A post a developer reads through because:

1. They recognize the pain in the first 2 pages.
2. The author shows technical chops fast (real code, real stack, real
   commands).
3. They can *see* the thing working (terminal output, architecture).
4. The voice has personality — dry wit, opinions, mild sarcasm — without
   being a marketing pitch or a beige philosophical essay.

If a developer would close the tab thinking *"yeah, I'd actually try
this,"* the skill did its job.

## What this skill is NOT for

- Pure marketing pitches (see counter-example #1 below)
- Pure philosophical essays with no code or visuals (see counter-example #2)
- Dev logs / debug stories (use `social/day-2-debug-and-ship.html` as the model)
- Feature launch announcements (use `social/post-templates.html`)

## Type discipline (this is where attempts kept failing)

This is the most-violated rule. Read it carefully.

| Use | Font | Notes |
|---|---|---|
| **Body text** (paragraphs, sentences) | **Kalam** | Handwritten *but readable*. The default for prose. |
| **Short accent** (1–3 word emphasis, callouts, labels) | Caveat | Flourish only. Never for paragraphs. Never for bullet lists. |
| **Code / terminal output** | JetBrains Mono | The default for any block that begins with `$` or contains commands. |
| **Stack callout headings** | Inter (sans-serif) | For visual variety — used on stack-card pages. |
| **Logo wordmark only** | Cormorant Garamond italic | *Banned for body text.* Two prior attempts used it for paragraphs and the result felt either marketing (too elegant) or beige (too plain). |

Load these from Google Fonts:

```html
<link href="https://fonts.googleapis.com/css2?family=Kalam:wght@300;400;700&family=Caveat:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;700&family=Inter:wght@400;500;600;700&family=Cormorant+Garamond:ital,wght@1,500&display=swap" rel="stylesheet" />
```

## Visual variety (this is the OTHER thing that kept failing)

A reflection-only essay with twelve pages of paragraphs is *plain*. Avoid
that. Every post in this style must include at least **three** of the
following, distributed across the carousel:

- A terminal block showing real `inode add` / `inode ask` output
- A second terminal block showing install / setup commands
- An architecture diagram (sketchy boxes + arrows; reuse `sk-rect` SVG
  symbol for the boxes, draw curved arrows with paths)
- A stack-card grid (3–6 callouts, each with name + one-line tagline)
- A side-by-side comparison page (e.g. "before / after", "obvious /
  ours")
- A sketchy embedding scatter (dots clustering by topic in a 2D box)
- A "tried everything else" page with hand-drawn ✗ marks next to short
  takes on the alternatives

If three of these aren't on the page list, the post is too plain.

## Page chrome — terminal-window styling

When you ship a terminal block, dress it as a real terminal window:

```html
<div class="term-frame">
  <div class="term-titlebar">
    <span class="lights"><span class="r"></span><span class="y"></span><span class="g"></span></span>
    <span class="label">~/notes — inode add</span>
  </div>
  <div class="term-body">
    <span class="term-line"><span class="pr">$</span>inode add ...</span>
  </div>
</div>
```

This gives: red/yellow/green traffic-light dots, a centred dark title
bar with a path/command label, and a paper-coloured body. Pages with
multiple terminals (e.g. install) use one frame per logical step with a
distinct title-bar label per frame ("step 1 — local LLM", "step 2 —
the binary", etc.).

ASCII rules `─── ─── ─── ─── ───` or `┄ ┄ ┄ ┄ ┄ ┄ ┄` (use the
`.ascii-rule` class, JetBrains Mono in pencil) make great section
dividers between content blocks on pages that aren't otherwise terminal
shaped — page 5 (alternatives), page 11 (honest part), page 12 (end
card).

A blinking-static cursor `<span class="cursor"></span>` after the last
command on the install page is a nice touch; don't overuse it.

## Layout — every page must vertically center

`.reading-block` must use `display: flex; flex-direction: column;
justify-content: center;` so content sits centered between header and
footer. Page 11 of the previous attempt was the only correctly-centered
page; every other page looked top-heavy. **Don't override `top`** with
absolute pixel offsets per page — let the flex centering do the work.

## Dual-font typography pattern (the visual texture you want)

The most-loved styling in v3 was a two-voice line: a Caveat-red phrase
followed by a Kalam-italic continuation, both on one line:

```
no, but seriously —    read the next page.
[Caveat 48px red]      [Kalam 28px ink-soft italic]
```

Use this pattern for transitions, callouts, captions — *the brand voice
speaks first in red, the narrator continues in gray*. Land it at least
3 times across the carousel (cover, mid-post transitions, end card).
Don't use it on every page; the rhythm matters.

## NO EMOJIS

Not 🐛 not ⭐ not 🤝 not 🚀 not even ironically. Not as meta-commentary
about not-using-emojis. Hand-drawn marks (the SVG `sk-x-mark`, the
unicode ✓ ✗ → ↳ · ─) are fine — they're typographic glyphs, not emojis.
Apple's ⌘ key symbol is fine in keyboard-shortcut contexts.

## Voice (the personality layer)

- **Dry wit, not sarcasm-as-spite.** Punch *across* (the SaaS-industry
  norm) or *up* (your own past attempts). Never *down* (people who like
  Notion are fine; Notion's memory model is the problem).
- **Confident opinions are required.** Hedging reads as marketing
  ("perhaps consider", "could be useful for"). State the take.
  *"Notion is a Notion replacement"* > *"Notion has limitations"*.
- **Self-aware about the genre.** Open by acknowledging the reader is
  about to scroll past another AI notes post. Earn the next two pages.
- **Real code earns trust.** Pseudo-code costs trust. Use real binary
  names, real flags, real outputs.
- **Take potshots at your own future versions.** *"please don't ask me
  to add a kanban board"* lands harder than *"future work includes
  evaluating richer document types."*
- **First person is fine.** Casual *I* and *you* both work — this is a
  developer talking to a developer.

## Banned phrases (still)

`10x`, `supercharge`, `unleash`, `game-changer`, `revolutionary`, `the
future of`, `blazingly fast` (unless ironic), `democratize`, `empower`,
`enterprise-grade` (unless ironic), `stop worrying about`, `say goodbye
to`, `join the [waitlist|community|movement]`.

## Required arc — 12 pages of developer-blog

1. **Cover.** A self-aware title. *"another notes app."* with a punch
   subtitle. No big underlined slogan.
2. **The bait.** "you've been here" — recognizable scenarios with
   hand-drawn ✗ marks (cmd-K in Notion failing, etc.). 4-6 lines max.
3. **The unbait.** Direct address: *"I know you scroll past 'AI notes'
   posts. so do I."* Earn the next 9 pages in 2 sentences.
4. **Show, don't tell — terminal #1.** `inode add` / `inode ask` with
   real, masked output. The fastest possible trust-builder.
5. **"I tried everything else"** — sharp takes on the obvious
   alternatives (Notion, Obsidian, 1Password). Hand-drawn ✗ marks fine.
6. **The trick — embeddings, plain words.** With a sketchy 2D scatter
   showing meaning-clustering. *"that's the whole trick."*
7. **Architecture diagram.** Sketchy boxes + arrows showing CLI →
   adapters → backends.
8. **The stack — opinionated callouts.** 5–6 items, each with a
   one-line tagline. Inter sans-serif headings for variety.
9. **What it isn't.** Confident, opinionated. *"not a Notion
   replacement; Notion is a Notion replacement."*
10. **Install — terminal #2.** Real commands, copy-pasteable.
11. **The honest part.** One quiet paragraph (Kalam body, no jokes).
    Why this exists. The only earnest page.
12. **End card.** *"MIT · no telemetry · you owe me nothing"* with a
    cheeky last line.

Combine or split as needed; ship 10–14 pages.

## Counter-examples (read these before writing)

The repo contains two prior attempts at this post type. Both failed
differently:

**Counter-example #1 — `social/for-the-ones-who-lose-things.html`** (now
removed). Drifted into marketing structure: benefits list ("what you
stop carrying"), audience segmentation page ("for whom"), CTA verbs
("try it · watch it · break it"). The voice was too soft and too
universal.

**Counter-example #2 — first \`what-we-hand-to-machines.html\`** (also
revised). Overcorrected into beige philosophy. All paragraphs in
Cormorant Garamond italic, no code, no diagrams, no opinions, no wit.
Read like a Medium post nobody finished.

The pattern that works is between these: **technical substance with
personality and visual variety**.

## Project facts you can draw on

- inode — a CLI knowledge base. Save anything; ask in plain English;
  get the right thing back.
- Stack today: Go · SQLite + sqlite-vec · Ollama (local LLM and
  embeddings) · AES-256-GCM at rest · OS keychain for keys.
- Ollama is the default — fully local, fully free. Voyage / Claude API
  paths exist but are optional.
- Built at zero cost. Local-first is the architecture, not aesthetic.
- Phase 1 (v0.1.0) shipped.

## Reference files
- `social/post-templates.html` — design system source of truth
- `social/day-2-debug-and-ship.html` — most recent dev log carousel,
  good reference for terminal-block styling
- `README.md` — neutral product description
