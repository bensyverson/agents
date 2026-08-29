# Going public — no project names in this repo

*2026-08-29.* This repo is published under the MIT license. Its history is rewritten once, at publication, and never again — so anything committed after that is public for good.

## What was found

A pre-publication audit found no secrets, but the dated design docs and the per-repo migration analyses named the managed repos freely, and one assessment line quoted a client dollar figure, a rate card and a proposal filename lifted from a managed repo's knowledge base. The tool's job is to compare shared rules against real repos, so evidence naturally arrives as "repo X's gotchas.md has 47 entries" — the leak is structural, not careless.

## Ruling

- **Managed repos are referred to by pseudonym or by shape, never by name**, in everything tracked here: plans, assessments, backlog, gotchas, test comments and fixtures. "A Go+web service with 47 gotchas" carries the evidence; the name does not.
- **Nothing lifted from a managed repo's content** — client names, figures, filenames, quoted rulings — is pasted here. Cite the shape of the finding.
- **Per-repo working material lives in `local/`**, which is gitignored. The migration analyses moved there.

The maintainer's own name appears only in the README byline and the license.
