# GWX ↔ GalaxyWars spec — 08 Alliances

Alliances, mined from `HAR-DTA` (`info/recordings/Uni5/`). Reference client
`GalaxyWars v3.16.19-beta`, universe `uni5`, planet `12:147:1` (`[M]`, player
`Redock`). Two alliances are observed: the player's own **wurst** (*Es Geht Um
Die Wurst*, 1 member, 3.530 pts) and the foreign **Linux** (*Die Killer
Pinguine*, 1 member, 22.642 pts, founder `Highkick_Hassan`).

The whole feature lives under `/app/<coord>/alliance/*`: a **public** area
(search, profile, apply, create) and an **internal** area (`/alliance/internal/*`
for members: dashboard, members, board, management).

## Public surfaces

| Route | Purpose |
|---|---|
| `GET /alliance/search` (`?page=N`) | browse/search all alliances |
| `GET /alliance/show/<tag>` | public alliance profile |
| `GET/POST /alliance/apply/<tag>` | apply form + submit |
| `GET /alliance/applications` (`?tag=<tag>`) | my sent applications |
| `GET /alliance/application-revoke/<uuid>` | withdraw an application |
| `GET/POST /alliance/create` | found a new alliance |

### Search / browse

`/alliance/search` renders a paginated `Name` table (20 per page, numbered
pagination + Previous/Next). Each row links to the profile and shows the name
plus the tag in brackets:

```
Die Killer Pinguine [Linux]      PoppR [PoppR]      new World order [nWo]  …
```

A `search_alliance` form (POST to the same URL) has a single required field
`search_alliance[term]` (placeholder *Search term*) plus `search_alliance[send]`
and the CSRF token. Header buttons: **Applications** and **Create alliance**.

### Public profile (`/alliance/show/<tag>`)

Three-row table + a description block:

| Field | Example (`Linux`) |
|---|---|
| Points | `22.642` |
| Members | `1` (+ **Bewerben**/*Apply* link when you are not a member) |
| Founder | `Highkick_Hassan` (links to `/player/<uuid>`) |

Below the table the free-text **description** is rendered (multi-line, HTML).
An alliance **logo/header image** (`img.img-fluid`, Resvg/`gw.ddl.re` URL) is
shown when set. The *Apply* link is suppressed on your own alliance.

### Applying

`GET/POST /alliance/apply/<tag>` — heading *"Bewerben bei <name>"*. The
`create_alliance_application` form is a single required `textarea`:

`POST /alliance/apply/<tag>` (form-urlencoded) → **302 /alliance/applications?tag=<tag>**:

```
create_alliance_application[message]=hallo
create_alliance_application[send]=
create_alliance_application[_token]=<csrf>
```

The redirect target shows an info banner *"Application sent."* and lists the
pending application: *"Application for [tag]"* + timestamp + message + a
**Withdraw** button. **Withdraw** → `GET /alliance/application-revoke/<uuid>`
→ **302 /alliance/applications** (no confirm page captured). `/alliance/
applications` lists all my open applications; empty → *"No applications
available."*

### Founding

`GET/POST /alliance/create` — two required text inputs (tag =
*Abbreviation/tag*, name = *Name*). No description/openness/min-points fields.

`POST /alliance/create` (form-urlencoded) → **302 /alliance/show/<tag>**:

```
create_alliance[tag]=wurst
create_alliance[name]=Es Geht Um Die Wurst
create_alliance[send]=
create_alliance[_token]=<csrf>
```

The creator becomes **Founder** (crown icon in the member list).

## Internal area (`/alliance/internal/*`)

The internal area is only reachable as a member; a sub-navigation bar links
**Overview | Members** plus **Board** and **Management**.

| Route | Purpose |
|---|---|
| `GET /alliance/internal` | dashboard |
| `GET /alliance/internal/members` | member list |
| `GET /alliance/internal/leave` | leave the alliance |
| `GET /alliance/internal/board` | shared board (thread list) |
| `GET/POST .../board/thread/create` | new thread |
| `GET .../board/thread/detail/<uuid>` | thread + replies |
| `GET/POST .../board/thread/edit/<uuid>` | edit thread |
| `GET .../board/thread/delete/<uuid>` | delete thread |
| `GET/POST .../board/thread/create-post/<thread-uuid>` | reply |
| `GET/POST .../board/thread/edit-post/<post-uuid>` | edit a reply |
| `GET /alliance/internal/management` | management index |
| `GET .../management/applications` | review applications |
| `GET .../management/log` | audit log |
| `GET .../management/ranks` | ranks |
| `GET .../management/ranks/create` | new rank |
| `POST .../management/ranks/sort` | reorder ranks (`ajax-sort`) |
| `GET/POST .../management/settings` | change description |
| `GET/POST .../management/settings/change-image` | upload logo |
| `GET .../management/settings/delete` | close alliance |

### Dashboard & members

`/alliance/internal` mirrors the public profile (points, members with a
**Leave** link, founder, description, header image). `/alliance/internal/members`
is a table:

| Column | Notes |
|---|---|
| Name | links to player profile; the founder gets a `fa-crown` icon |
| Points | member score |
| Rank | rank name (`Founder` for the founder) |
| Last online | e.g. `online` (or a timestamp) |
| actions | empty for the sole founder in this capture |

### Board

`/alliance/internal/board` lists threads with a **Thema erstellen**/*Create
topic* button. `GET /board/thread/create` offers a `create_thread` form:

- `create_thread[title]` — required, **min 3 characters**
  (invalid → 200 re-render with *"This value is too short. It should have 3
  characters or more."*),
- `create_thread[message]` — required textarea,
- `create_thread[sticky]` — checkbox *"Pin topic to the top"* (`value=1`),
- `create_thread[send]`, `create_thread[_token]`.

`POST /board/thread/create` → **302 /board/thread/detail/<uuid>**:

```
create_thread[title]=Das ist ein toller title
create_thread[message]=Genau
create_thread[send]=
create_thread[_token]=<csrf>
```

A thread detail shows the **title**, each post as `messages-detail > 
conversation-message` with author, relative time (`heute 06:57:52`) and body,
plus **edit**/**delete** controls on the thread and **edit-post** on each post,
and a **Beitrag erstellen**/*Create post* button
(`.../board/thread/create-post/<thread-uuid>`) for replies. Board moderation =
these edit/delete routes; no separate moderator UI is observed.

### Management

`/alliance/internal/management` is the index of the management area, whose
sub-nav is **Applications | Audit log | Ranks | Settings**.

- **Applications** (`/management/applications`): lists pending membership
  applications (empty here: *"No applications available."*). Accept/reject
  actions exist conceptually but were not exercised in the capture.
- **Audit log** (`/management/log`): table `Date | Who | Type` (empty in this
  capture — a fresh alliance).
- **Ranks** (`/management/ranks`): a `data-controller="ajax-sort"` list whose
  drag handle class is `.grab`; reordering POSTs to `/management/ranks/sort`.
  A **New rank** button links to `/management/ranks/create`. The captured
  alliance has **no ranks defined** (empty list), and `/ranks/create` is not in
  the capture, so the concrete permission/rights checkboxes are **not
  observable** from this HAR (only the rank CRUD + sort endpoints are).
- **Settings** (`/management/settings`): *Change description*, *Change image*,
  *Close alliance*.

### Settings POSTs

Change description — `POST /management/settings` (form-urlencoded) → **200**
(re-renders settings, no redirect):

```
change_alliance_description[description]=Incredible cool Alliance!
change_alliance_description[send]=
change_alliance_description[_token]=<csrf>
```

Change image — `GET /management/settings/change-image` renders a
`cropped-image-upload` cropper sized **1520 × 855**, accepting `.jpg/.jpeg/.png`.
`POST .../settings/change-image` is `multipart/form-data` → **302 /alliance/
internal**:

```
change_alliance_image[imageFile]=<file>   (image/webp observed)
change_alliance_image[save]=
change_alliance_image[_token]=<csrf>
```

Close alliance — *"Close alliance"* button links to
`GET /management/settings/delete` (confirm/execute pages not captured).

## Observed POST field summary

| Form | Endpoint | Fields | Result |
|---|---|---|---|
| `search_alliance` | `POST /alliance/search` | `term`, `send`, `_token` | list/page |
| `create_alliance_application` | `POST /alliance/apply/<tag>` | `message`, `send`, `_token` | 302 `…/applications?tag=` |
| `create_alliance` | `POST /alliance/create` | `tag`, `name`, `send`, `_token` | 302 `…/show/<tag>` |
| `create_thread` | `POST …/board/thread/create` | `title`, `message`, `sticky`, `send`, `_token` | 302 `…/thread/detail/<uuid>` |
| `change_alliance_description` | `POST …/management/settings` | `description`, `send`, `_token` | 200 |
| `change_alliance_image` | `POST …/settings/change-image` | `imageFile` (multipart), `save`, `_token` | 302 `/alliance/internal` |

## Rank / rights model

The capture only proves the **structure** of the rank model, not the permission
inventory: ranks are a per-alliance, **ordered** list (drag-sort via
`/ranks/sort`) with create/edit routes, and every member carries exactly one
`Rank` (the founder's is `Founder`, hard-wired). Because the reference alliance
had no ranks configured and `/ranks/create` was never fetched, the set of
grantable rights could not be read from this HAR.

## Endpoint summary

| Method | Route | Purpose |
|---|---|---|
| GET | `/alliance/search` | browse (paged) |
| POST | `/alliance/search` | search by term |
| GET | `/alliance/show/<tag>` | public profile |
| GET/POST | `/alliance/apply/<tag>` | apply |
| GET | `/alliance/applications` | my applications |
| GET | `/alliance/application-revoke/<uuid>` | withdraw |
| GET/POST | `/alliance/create` | found alliance |
| GET | `/alliance/internal` | dashboard |
| GET | `/alliance/internal/members` | members |
| GET | `/alliance/internal/leave` | leave |
| GET/POST | `/alliance/internal/board/thread/create` | new thread |
| GET | `/alliance/internal/board/thread/detail/<uuid>` | thread |
| GET/POST | `…/thread/edit/<uuid>`, `…/thread/delete/<uuid>` | thread moderation |
| GET/POST | `…/thread/create-post/<uuid>`, `…/thread/edit-post/<uuid>` | replies |
| GET | `/alliance/internal/management` | management index |
| GET | `…/management/applications` | review applications |
| GET | `…/management/log` | audit log |
| GET | `…/management/ranks` (`/create`, `/sort`) | ranks |
| GET/POST | `…/management/settings` | change description |
| GET/POST | `…/management/settings/change-image` | logo upload |
| GET | `…/management/settings/delete` | close alliance |

## Differences vs gwx (what implementing this needs)

gwx has **no alliance subsystem at all**. A faithful minimal port adds:

1. **Tables:** `alliances` (tag unique, name, description, image, points,
   founder, created_at, status) and `alliance_members` (alliance, player,
   rank, joined_at).
2. **Applications:** `alliance_applications` (uuid, alliance, applicant,
   message, created_at, status) with apply → withdraw, plus the internal
   **accept/reject** review (accept creates the membership).
3. **Board:** `alliance_board_threads` (uuid, alliance, title, sticky, author,
   created_at) and `alliance_board_posts` (uuid, thread, author, body,
   created_at), with thread/post edit + delete and a `sticky` pin.
4. **Ranks/rights:** `alliance_ranks` (uuid, alliance, name, position,
   rights bitmask) + a `rank_id` on members; the founder rank is built-in and
   holds all rights. Rights gate the management and board-moderation routes.
   (Define the right set for gwx — not pinned by the reference HAR.)
5. **Public pages:** `/alliance` search (paged), `/alliance/<tag>` profile,
   apply and create forms; **internal pages** for dashboard, members (with
   *Leave*), board, management (applications, log, ranks, settings) — all
   server-rendered GET/POST, no LiveComponent.
6. **Audit log:** `alliance_log` (timestamp, actor, type) written by
   membership/rank/settings/board actions, rendered as `Date | Who | Type`.
7. **Image upload:** one alliance logo per alliance (crop 1520 × 855, webp/jpg),
   shown on profile + dashboard.
