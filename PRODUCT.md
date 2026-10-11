# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Go, server-rendered HTML (`net/http`, `html/template`), SQLite, single Docker container. Chosen by the owner. Public read pages ship no JavaScript other than the optional Umami tracker; authenticated pages may load a lightweight rich-text editor.

## Users

- **Readers:** people who tap a paste link in a post on X.com, mostly on a phone, often on mobile data. They chose to open a spoiler and want the text immediately. They have no account and never need one.
- **Authors:** the instance owner and any users the owner admits. They write short-to-medium prose, typically thoughts about a film whose plot they do not want to spoil in the post itself, then copy the link into X.
- **Admin:** the owner, who controls whether sign-ups are open and manages users.

## Product Purpose

pastr shares text behind a URL so that a spoiler never appears in the social post that links to it. Success is a reader who gets from tap to reading with no friction, and an author who gets from idea to copied link in under a minute.

## Positioning

A pastebin built for prose and spoilers, not code: the link preview never leaks the body, the author chooses whether the text shows at once or behind a tap, and a paste can expire or burn after a set number of views.

## Operating Context

- Links are shared on X.com; the preview card shows only the author-written, spoiler-safe title.
- Self-hosted on the owner's domain; published as an open-source project for others to self-host.
- Analytics go to an Umami instance the owner already runs, with custom events (reveal taps per paste, copy actions, outbound links, expired or burned link hits, author actions).

## Capabilities and Constraints

- Authenticated create, read, update and delete of pastes; username and password sign-in.
- Sign-ups can be enabled or disabled by the admin; disabled by default.
- Per paste: title, rich-text body, reveal mode (immediate or tap to reveal), optional expiry, optional view limit.
- Rich text is limited to basic formatting and sanitised on the server; no images, embeds or scripts.
- Authors may set an optional display name and link, shown as a small byline; blank by default.
- No public index of pastes; pastes are unlisted and not indexed by search engines.
- Lean by intent: features outside this list need a reason.

## Brand Commitments

- Name: pastr.
- New identity; no existing brand to match.
- Must not look like a developer tool (no monospace-everywhere, terminal or code-paste styling).
- Must not look like generic SaaS; the owner asked to be surprised.
- No chrome on the read page: nothing competes with the text.
- Nothing may slow the read: no web fonts, images or animation that delay text on a phone.

## Evidence on Hand

No existing content, logo or assets. Example pastes used in design work are synthetic.

## Product Principles

1. The reader's time comes first: text appears before anything else.
2. The spoiler never leaks: not in the preview card, not before the reader asks.
3. Lean over capable: each feature must earn its place.
4. Private by default: no reader data stored, nothing listed publicly.
5. Easy to self-host and easy to read as code.

## Accessibility & Inclusion

WCAG 2.2 AA. The read page must work without JavaScript, respect reduced-motion and system light/dark preferences, and remain readable at 200% zoom.
