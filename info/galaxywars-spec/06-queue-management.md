# GWX ↔ GalaxyWars spec — 06 Queue management

Universal pattern: an `append` action adds an order and returns `302` back to the page; a `remove/<id>` action cancels it. Every append is per-scope (one active order per building/research/unit scope).

| Scope | Append | Remove | Notes |
|---|---|---|---|
| Building | `/building/append/<ident>` | `/building/remove/<id>` | `id=1634393` observed — orders are addressable by numeric id. |
| Research | `/research/append/<ident>` | `/research/remove/<id>` | `id=88648` observed. |
| Ships/Defense | `POST /ship` (batch form) | — | `add_combat_units_to_queue[combatUnits][<ident>]` per ship; a single batch submit enqueues all non-zero quantities. |


The shipyard/queue UI exposes **per-item checkboxes** with **Delete selected** and **Delete all** behind a confirmation popup (batch-delete semantics; per the capture-runbook note). The removal endpoints above are the underlying single-order form.

## Mode of operation

- `POST /app/push-stats` (2-byte JSON) is polled to refresh the header resource/queue state — the client's lightweight status endpoint.

