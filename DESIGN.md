---
name: pastr
description: Poster Wall. Every paste is a poster in a series - one ink, one signal colour, one bar.
colors:
  ground: "#ffffff"
  ink: "#111111"
  ink-soft: "#5a5a5a"
  red: "#d52b1e"
  blue: "#0047bb"
  green: "#00703c"
  orange: "#c2410c"
  ground-dark: "#0d0d0d"
  ink-dark: "#f5f5f5"
  ink-soft-dark: "#a8a8a8"
  red-dark: "#ff5a4d"
  blue-dark: "#6ea1ff"
  green-dark: "#3ecf8e"
  orange-dark: "#ff8a3d"
typography:
  display:
    fontFamily: "'Helvetica Neue', Helvetica, Arial, system-ui, sans-serif"
    fontSize: "clamp(2.5rem, 9vw, 6rem)"
    fontWeight: 700
    lineHeight: 0.98
    letterSpacing: "-0.03em"
  headline:
    fontFamily: "'Helvetica Neue', Helvetica, Arial, system-ui, sans-serif"
    fontSize: "2.25rem"
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: "-0.02em"
  title:
    fontFamily: "'Helvetica Neue', Helvetica, Arial, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.01em"
  body:
    fontFamily: "'Helvetica Neue', Helvetica, Arial, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: "'Helvetica Neue', Helvetica, Arial, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 700
    lineHeight: 1.55
rounded:
  none: "0"
spacing:
  unit: "0.5rem"
  rule: "2px"
  bar: "0.75rem"
  gutter: "clamp(1.25rem, 5vw, 4rem)"
  measure: "66ch"
components:
  bar:
    backgroundColor: "{colors.red}"
    height: "{spacing.bar}"
  button:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0 1.5rem"
    height: "3rem"
  button-hover:
    backgroundColor: "{colors.red}"
    textColor: "{colors.ground}"
  reveal-control:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    typography: "{typography.title}"
    rounded: "{rounded.none}"
    padding: "0 1.25rem"
    height: "3.5rem"
  reveal-control-hover:
    backgroundColor: "{colors.red}"
    textColor: "{colors.ground}"
  input:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "0.6rem 0.75rem"
    height: "3rem"
  form-error:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    typography: "{typography.label}"
    padding: "0.75rem 1rem"
---

# Design System: pastr

## Overview

**Creative North Star: "Poster Wall"**

pastr looks like a wall of concert posters in the Swiss International Style. Every page is one sheet: a large bold headline set flush left, a thick bar of colour directly under it, and plain text on a single grid. Each paste gets its own colour, so a series of pastes reads as a series of posters by the same printer.

The system is small on purpose. There is one ink, one ground, one typeface stack, one rule weight and one bar. Controls are typographic: square, ink-filled blocks and underlined words. There are no cards, shadows, gradients, icons or images. Whitespace and the size jump between headline and text do the work that decoration does elsewhere.

The whole system lives in one stylesheet, `internal/web/static/app.css`. The token names used below (`--ink`, `--signal`, `--unit` and so on) are the custom properties declared at the top of that file.

**Key Characteristics:**
- One big flush-left headline over one coloured bar, on every page.
- One signal colour per paste, picked from four, fixed for the life of the paste.
- Square corners, 2px ink rules, flat surfaces.
- System grotesque type only; no web fonts and no images.
- Light and dark follow the system setting; there is no theme toggle.
- The read page is text first: no application JavaScript, and the body text does not move.

## Colors

Black, white and one loud colour at a time. The values are in the frontmatter; the `-dark` entries are what the same custom properties resolve to under `prefers-color-scheme: dark`.

### Primary
- **Signal** (`--signal`): not a colour of its own but a pointer to one of the four below. The server derives it from the paste's slug (`paste.SignalColour`) and sets a `signal-red|blue|green|orange` class on `<body>`, and on each row of the paste list. Pages that are not about a paste (sign in, account, admin, errors) default to red. Signal fills the bar, the hover state of ink blocks, the focus ring, the text selection, the caret and the underline of a hovered link.
- **Poster Red, Poster Blue, Poster Green, Poster Orange** (`--red`, `--blue`, `--green`, `--orange`): the four signal colours. Each has a deep value for the white ground and a brighter value for the black ground.

### Neutral
- **Ground** (`--ground`): the sheet. Pure white in light, near-black in dark. Also the text colour on ink and signal fills.
- **Ink** (`--ink`): all text, all rules and borders, and the fill of primary controls.
- **Soft Ink** (`--ink-soft`): secondary text only: meta lines, hints, placeholders, and destructive links at rest.

### Named Rules
**The One Signal Rule.** A page shows one signal colour. Never place two of the four together, except in the paste list (each row carries its own paste's colour) and the decorative living poster (all four).

**The Fill-Not-Text Rule.** Signal is a fill, a ring or an underline. Text is always ink, soft ink or ground. There is no red for errors and no green for success: an error is an ink block with ground text, a notice is an outlined box, and a destructive link is quieter than its neighbours, not louder.

**The System Theme Rule.** Dark mode is a `prefers-color-scheme` media query that reassigns the same custom properties. Do not add a toggle, a theme class or per-component dark overrides; change the token.

## Typography

**Display, body and label font:** `"Helvetica Neue", Helvetica, Arial, system-ui, sans-serif` (`--font`). One stack for everything.

**Character:** a neutral grotesque used at two extremes, very large and bold or small and plain, always flush left and ragged right. Using only installed fonts is an owner constraint, so that nothing delays text on a phone.

### Hierarchy
- **Display** (`--step-3`, 700): the poster headline on the read, gone and error pages and the title field in the editor. It scales with the viewport, balances its lines and may break anywhere so long titles and URLs never overflow.
- **Headline** (`--step-2`, 700): page titles on signed-in pages (My pastes, Account, Admin, Sign in, Link ready).
- **Title** (`--step-1`, 700): section headings, paste titles in the list, the reveal control and the empty-state sentence.
- **Body** (`--step-0`, 400): paste text and form input text. The reading column is capped at `--measure` (66ch).
- **Label** (`--step--1`): field labels, navigation and action links at 700; meta lines, hints and help text at 400 in soft ink. Numbers in meta lines use tabular figures.

### Named Rules
**The Two Weights Rule.** Only 400 and 700 are used. Hierarchy comes from size and weight, never from colour, capitals or letter-spacing. Negative tracking is applied to large sizes only.

**The One Stack Rule.** No web fonts, no second family, no monospace. pastr is for prose and must not look like a code tool.

## Layout

Everything hangs from the left edge. `main` has a fluid side gutter (`--gutter`) and content is never centred. Spacing is a multiple of `--unit` (0.5rem); the common steps are 2, 3, 4, 5, 6 and 8 units.

Page widths: 46rem for posters and the editor, 60rem for lists, 72rem for the read and gone pages, 28rem for forms.

Layouts are single column by default and gain columns as room allows:
- **40rem:** list rows put their actions in a second column beside the title and meta.
- **48rem:** the three paste settings in the editor sit side by side.
- **60rem:** sign-in and sign-up put the form on the left and the living poster on the right, divided by an ink rule.
- **64rem:** the read page becomes a two-column grid. Headline and bar span both; the meta moves into a narrow 13rem left column under a short ink rule and turns bold ink; the text sits in the right column at the 66ch measure.

Interactive targets are at least 2.75rem tall (links, radios, toolbar buttons), 3rem for buttons and inputs, and 3.5rem for the reveal control.

## Elevation & Depth

Flat. There are no shadows, blurs, gradients or layered surfaces. Separation comes from 2px ink rules and from whitespace. The only overlap in the system is deliberate and graphic: on the gone page the bar slides up and strikes through the headline, blended (`multiply` in light, `screen` in dark) so the letters stay readable through it.

## Shapes

Rectangles only. Every control, field and box has square corners (`border-radius: 0`), including radio buttons, which are drawn as square ink boxes. Lines are 2px ink (`--rule`); the bar is 0.75rem (`--bar`). The one curve in the system is the half ring in the living poster, which is decoration and not a control.

## Components

### The bar
A full-width block of signal colour, 0.75rem tall, placed 3 units under the headline. It appears once per page and is hidden from assistive technology. In the paste list a shorter version (5rem wide, 0.375rem tall) sits above each title at half length and extends to full length on hover or focus.

### Poster head and meta
Headline, bar, then one small meta line in soft ink (byline in bold ink if the author set one, date, time left). The read page carries nothing else: no navigation, no footer, no branding.

### Reveal control
A native `<details>`/`<summary>`, so it works without JavaScript.
- **Closed:** a full-width ink block, 3.5rem tall, reading "Show the text" in Title type. Hover fills it with signal.
- **Open:** it shrinks to a small underlined soft-ink link, "Hide the text", above the text.

### Buttons
- **Primary (`.button`):** ink fill, ground text, weight 700, square, 3rem tall, 1.5rem side padding, left-aligned in its container. One per form.
- **Hover:** fill changes to signal. **Active:** moves down 1px. **Copied:** fill holds on signal and the label reads "Link copied" for two seconds.
- **Text action (`.link`):** bold underlined ink with a 2px underline; the underline turns signal on hover. Used for secondary actions (Open, Copy link, Sign out).
- **Destructive (`.link--danger`):** the same, in soft ink, turning ink on hover.

### Inputs and fields
- **Style:** ground fill, 2px ink border, square, 3rem minimum height, body type. A bold label sits above; help text sits below in soft ink.
- **Focus:** every focusable element gets the same ring: 3px solid signal, offset 3px.
- **Radios:** 1.25rem square ink boxes; the checked one fills with ink inside a ground inset, and its label turns bold.
- **Title field:** in the editor the title is a borderless textarea set in Display type, so the author writes directly on the poster.
- **Error:** an ink block with bold ground text above the form. **Notice:** an outlined box that begins with a bold "Done.".

### Navigation
Signed-in pages only. A single row of bold label-size links above a 2px ink rule, with Sign out pushed to the right. Links have no underline at rest and a signal underline on hover. It wraps on narrow screens; there is no menu button.

### Index list
Rows separated by 2px ink rules. Each row has a Title-size heading, a soft-ink meta line and bold text actions. Each paste row carries its own signal class.

### Editor
The Trix editor is vendored and restyled only from `app.css`: toolbar groups become square ink-outlined boxes with 2px dividers, and the writing area is an ink-outlined box set to the reader's measure so line breaks match the read page. Code blocks, attachments and nesting are removed in `editor.js`.

### Living poster
Three bars and a half ring in the four signal colours, drifting slowly. It is pure decoration (`aria-hidden`) and appears only on sign-in, sign-up and the empty paste list. It never appears on the read page.

### Motion
Motion is optional: every page is complete without it. All keyframe animations and view transitions sit inside one `@media (prefers-reduced-motion: no-preference)` block at the end of `app.css`, and the one scripted effect checks the same preference. The shared easing is `--ease` (`cubic-bezier(0.16, 1, 0.3, 1)`).

- **Bar draw (0.5s):** the bar grows from the left on load. On the gone page it then slides up to strike the headline.
- **Title rise (0.3s):** the headline on the read and gone pages is unmasked from below.
- **Reveal wipe (0.42s):** when "Show the text" is tapped, a full-screen sheet of signal colour crosses left to right. The text switches from hidden to visible while the sheet covers the screen. This is CSS on `details[open]`; no script is involved.
- **Copied (0.5s):** the button widens slightly once.
- **Typeset URL:** on the link-ready page the URL is set one character at a time (12ms each) by `events.js`.
- **Shared title:** cross-page view transitions are on, and a paste's title travels from its list row to the editor headline.
- **Living poster:** the shapes drift over 22 to 40 seconds, looping.

**The Still Text Rule.** Paste body text is never animated: no fade, slide, stagger or scroll effect. It is either hidden or fully there.

## Do's and Don'ts

### Do:
- **Do** use the existing custom properties for every colour, size and space. If a value is missing, add a token; don't write a literal.
- **Do** start every page with a headline and one bar, flush left.
- **Do** keep corners square and lines at 2px ink.
- **Do** put any new animation inside the `prefers-reduced-motion: no-preference` block, and make sure the page works with it removed.
- **Do** put all styling in `app.css` and all behaviour in the static script files. The Content Security Policy (`style-src 'self'`) blocks inline `style` attributes, `<style>` blocks and inline scripts; only the editor pages get a style nonce, for Trix.
- **Do** keep targets at least 2.75rem tall and keep the 3px signal focus ring on everything focusable.
- **Do** check both themes: a new colour needs a light and a dark value that each meet WCAG 2.2 AA against their ground.

### Don't:
- **Don't** add web fonts, a second typeface or monospace text.
- **Don't** add images, icons, illustrations or logos. The sheet is type and rectangles.
- **Don't** add a fifth signal colour, tints of the four, or a second signal colour on a page.
- **Don't** use signal colour for text, or colour to mean success, warning or danger.
- **Don't** add shadows, gradients, blur, rounded corners or card containers.
- **Don't** centre layouts or headlines.
- **Don't** add a theme toggle or store a theme preference.
- **Don't** load `events.js`, the editor or any new script on the read page, and don't add navigation, footers or other chrome to it. The only script it may carry is the instance owner's optional Umami tracker.
- **Don't** animate paste body text, and don't use the living poster on the read page.
- **Don't** edit the vendored Trix files; restyle the editor from `app.css`.
