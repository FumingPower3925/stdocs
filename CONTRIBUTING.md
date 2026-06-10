# Contributing to stdocs

Thanks for your interest in `stdocs`. This document covers the day-to-day
mechanics of contributing: how to run the tests, how to file issues, and how
to add a new translation.

## Development setup

```bash
git clone https://github.com/FumingPower3925/stdocs
cd stdocs
go test -race -count=1 ./...
golangci-lint run ./...
```

Requirements:

- Go 1.26.4 or later (the module's `go` directive; CI derives its toolchain from `go.mod`).
- `golangci-lint` v2.12.2 (matches CI; `brew install golangci-lint` on macOS).

## Running the tests

The main module has no third-party runtime or test dependencies. Two
test runs cover the project:

```bash
# Unit + race tests + fuzz the pattern parser
go test -race -count=1 ./...
go test -fuzz=^FuzzParsePattern$ -fuzztime=10s ./internal/pattern/

# YAML round-trip — this is a SEPARATE go module so that gopkg.in/yaml.v3
# never appears in the main module's dep graph. It is not run by the
# plain `go test ./...` above.
cd internal/spec/yaml/roundtrip_test && go test ./...
```

## Filing issues

Open an issue at <https://github.com/FumingPower3925/stdocs/issues>. Bug
reports should include a minimal reproduction; feature requests should
explain the use case, not just the proposed API.

## Pull requests

- Keep changes focused; one concern per PR.
- Update the README, CHANGELOG, and godoc comments as needed.
- All four CI jobs (`Test`, `Lint`, `YAML Roundtrip`, `Coverage`)
  must pass before review.

## Translations

The English `README.md` is canonical. Translations live alongside
it as `README.<lang>.md` files at the repo root and are linked
from the **Languages** line at the top of `README.md` (GitHub does
not auto-detect translated READMEs). Currently maintained
translations:

- `README.es.md` — Spanish (Español)
- `README.ca.md` — Catalan (Català)

### Requirements

You must satisfy **all** of the following to add or update a translation:

1. **You are a fluent speaker of the target language.** Native or
   near-native proficiency is required. The maintainers of `stdocs` do
   not speak every language the project is translated into, and we
   cannot verify the accuracy of a translation we cannot read. We
   rely entirely on the translator to be fluent.

2. **You do not submit machine-translated output.** Translations
   produced by Google Translate, DeepL, ChatGPT, Claude, or any
   other automated system — with or without human review — are not
   accepted as primary submissions. The project requires
   human-written translations. A translator may consult a
   machine-translation tool as a reference (e.g. to recall a
   technical term), but the resulting prose must be the translator's
   own and reflect idiomatic usage in the target language.

3. **You agree to maintain the translation.** Translations fall out
   of date as the English README evolves. When the English README
   changes in a way that affects a translated section, the named
   translator is expected to update the translation in the same
   pull request or in a follow-up within a reasonable window. A
   translation that falls more than one minor version behind the
   English README, and for which no maintainer is available, will
   have its link removed from the language list in the root
   `README.md`.

These rules are non-negotiable. Pull requests that appear to be
unreviewed machine output will be closed without review.

### Adding a new language

1. Open a GitHub issue in this repository describing the language,
   why a translation is needed, and naming the fluent speaker(s) who
   will maintain it. Wait for a maintainer to confirm.

2. Copy the current `README.md` to `README.<lang>.md` at the
   repo root, where `<lang>` is the two-letter [ISO 639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)
   code (`es`, `fr`, `de`, `ja`, …), or an [IETF BCP 47 language
   tag](https://en.wikipedia.org/wiki/IETF_language_tag) (`pt-BR`,
   `zh-CN`, …) when regional variation matters.

3. Translate the body. The community-translation banner at the top
   of the file stays **in English** so every reader sees the same
   canonical-link notice, regardless of language.

   Code blocks must stay identical to the English README, with one
   exception: code comments and user-facing example strings (API
   titles, `doc:` tags, summaries) should be localized. Identifiers,
   route patterns, URLs, commands, and numbers must not change.

4. In the same pull request, add a one-line entry to the language
   list at the top of the root `README.md` and a row to
   `TRANSLATORS.md` with your GitHub handle (a maintainer confirms
   it during review).

5. Open the pull request. A maintainer will review. Reviewers may
   request changes for tone, accuracy, or to match a project-wide
   terminology decision.

### Keeping your translation in sync

There is no automatic sync. When the English `README.md` changes in
a way that affects a translated section (new feature, renamed
option, new UI), the named translator is responsible for updating
`README.<lang>.md`. The English README is the source of truth;
translations are best-effort mirrors.

## Releasing

Releases are cut by the project maintainers from `main`. The release
process moves the `[Unreleased]` entries in `CHANGELOG.md` into a
new version section, tags the commit, and publishes a GitHub Release. Contributors do not need to cut
releases.
