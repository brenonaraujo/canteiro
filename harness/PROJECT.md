# Project overlay

This file is the **project** contract. `harness/bootstrap.md` stays
the framework invariant. Do not replace framework rules with stack defaults.

- **Name:** Canteiro
- **Slug:** canteiro
- **Domain:** canteiro (`domain-expert-canteiro`)
- **GitHub:** https://github.com/brenonaraujo/canteiro
- **Public host:** https://canteiro.brenon.cloud
- **Stack:** Go + Gin + PostgreSQL (versioned SQL migrations; no AutoMigrate) + Nuxt 4 / Nuxt UI / Pinia + Stripe + Google sign-in
- **CI kind:** full (Go/Nuxt modular CI)
- **Loop:** Hermes cron pooling (no GitHub webhook). See `harness/workflow/07-hermes-loop.md`.
- **team-manager:** orchestrates only. Never implements feature code.
- **Issue 0:** Bootstrap: functional spec + harness loop (canteiro)

## DoR
Spec in `docs/SPEC.md`, specialized domain-expert, labels, CI, `gmh loop doctor` green.

## DoD
Sensors green, PR with "Como testar".
Human `validado` is **waived** (test-lab): `qa` + MERGEABLE + CI green → squash merge, `done`, close. Do not idle waiting for a comment.

Delivery does not count as released until `https://canteiro.brenon.cloud` serves the product (not a zone 404).
