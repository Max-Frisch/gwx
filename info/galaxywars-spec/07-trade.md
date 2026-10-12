# GWX ↔ GalaxyWars spec — 07 Trade & marketplace

Player trading, mined from `HAR-DTA` (`info/recordings/Uni5/`). Reference client
`GalaxyWars v3.16.19-beta`, universe `uni5`, planet `12:147:1` (`[M]`). Endpoint
family `/app/<coord>/trading/*` plus the LiveComponent
`POST /app/_components/<coord>/OfferOverview`.

Trading is a **player-to-player marketplace**: you post an offer (give X, want Y
at a rate), another player **partially or fully accepts** it, and the goods are
physically moved by a **trading fleet** (travel time applies). A **Trading Post**
building is required; its level sets the per-planet offer slots.

## Tabs

| Tab | Route | Contents |
|---|---|---|
| Marketplace | `/trading` | all live offers (LiveComponent `OfferOverview`), filters + sort |
| My offers | `/trading/own` | your live offers; `Create offer` + `History` buttons |
| History | `/trading/own/history` | completed transactions (both sides) |
| Storage | `/trading/storage` | goods pending collection after a trade |
| Information | `/trading/information` | rate ratios, limits, fee schedule, public extreme-trade log |

`/trading/create`, `/trading/offer/<uuid>/detail`, `/trading/offer/<uuid>/revoke`
and `/trading/information/history` are sub-routes.

## Offer schema

An offer is: **give** `offeredResourceIdent` × `offeredResourceVolume`; **want**
`requestedResourceIdent` at a rate `course`; demand is derived as
`offeredVolume × course`.

- Resources: `iron`, `lutinum`, `water`, `hydrogen`.
- **Visibility**: *Public* (anyone) or *Private* (directed at a trading partner;
  `/trading/create?target=<player-uuid>` shows a "Trading partner" header).
- **Rate (`course`)**: see below.
- **Expiry**: public 7 days, private 6 hours (from the fee table).
- **Fee**: charged on create (2% offer fee) and on completion (seller/buyer).

### Rate / course (exact math from the client JS)

The client loads a per-resource `course` value (a JSON attribute on the create
form: `data-create-offer-course-value`):

| Resource | course value | meaning |
|---|---|---|
| Hydrogen | 1.00 | base |
| Lutinum | 1.26 | |
| Iron | 2.26 | |
| Water | 333.94 | |

A **higher number ⇒ cheaper per unit** (it is "units per 1 unit of hydrogen
value"). The **fair course** for `offered → requested` is

```
fairCourse = courseValue[requested] / courseValue[offered]        (rounded to 3 dp)
```

and the offer's rate slider may deviate **±50 %** (`max-course-deviation`):

```
minCourse = fairCourse / 1.5      maxCourse = fairCourse * 1.5
demand    = offeredVolume * course
```

Examples: `Iron→Hydrogen` fair `= 1/2.26 = 0.442` (range `0.295 … 0.664`);
`Iron→Water` fair `= 333.94/2.26 = 147.76`; `Lutinum→Hydrogen = 0.794`.

**Marked trade:** sliding into the outer 40 % on either side (`≤ -40` or `≥ +40`
on the `-50…+50` slider) reveals the warning *"Trade will be marked public"* and
the offer is published to the public **extreme-rate log**
(`/trading/information/history`). Offer detail shows *"This trade is marked."*
for such offers.

## Create offer

`GET /trading/create` (optionally `?target=<player-uuid>` for a private offer).

Form `create_offer` (Symfony): two resource `select`s + a swap icon, volume
`number` input with quick-add badges **All / +10.000 / +50.000 / +100.000**, a
rate `range` (min −50, max 50, step 1) with badges **−20 % / −10 % / 0 % / +10 % /
+20 %**, a free-text `course` input, plus read-only **Demand** and **Fee**
fields. `All` computes `floor(stock / 1.02)` (i.e. reserves the 2 % offer fee).

`POST /trading/create` (form-urlencoded) → **302**:

```
create_offer[offeredResourceIdent]=iron
create_offer[requestedResourceIdent]=hydrogen
create_offer[offeredResourceVolume]=10000
create_offer[course]=0,480
create_offer[send]=
create_offer[_token]=<csrf>
```

## Accepting (buy)

`GET/POST /trading/offer/<uuid>/detail`. A public offer shows the accept form
`create_transaction` (your own offers instead show *"Your own offers cannot be
accepted."*). Accept controller values: `fee = 0.015` (1.5 %),
`course`, `volume = offeredVolume`.

- Enter **Demand** (amount of the requested resource you take — **partial
  accepts allowed**); the form derives Offer, Fee and Net live (net = amount
  credited after fee). Quick-add **All / +10.000 / +50.000 / +100.000**.
- Detail shows visibility, *"Valid until <date>"*, and **Flight time** (the
  trading fleet's travel duration).

`POST /trading/offer/<uuid>/detail` (form-urlencoded) → **302**:

```
create_transaction[volume]=33113
create_transaction[send]=
create_transaction[_token]=<csrf>
```

## Revoke

`GET /trading/offer/<uuid>/revoke` behind a confirm dialog — *"Do you really
want to revoke this offer? Fees will be charged."* → **302**. (Fees are kept.)

## Fees & durations (`/trading/information`)

| | Public | Private |
|---|---|---|
| Offer fee | 2.00 % | 2.00 % |
| Seller fee | 1.50 % | 1.50 % |
| Buyer fee | 1.50 % | 1.50 % |
| Expiration fee | 1.50 % | 3.00 % |
| Offer duration | 7 Days | 6 Hours |

The offer fee is charged up-front (baked into `All`); seller/buyer fees apply on
completion; the expiration fee applies when an offer expires unaccepted.

## Storage & history

- **Storage** (`/trading/storage`): table `Resource | Expiration date`, with a
  **Credit all** button. Holds resources delivered by an accepted trade until
  collected; each entry expires.
- **My-offers history** (`/trading/own/history`): `Date | Resources | From |
  Resources | To`, with `From`/`To` linking to player profiles. Example:
  `12.10.2026 06:53:46 | 32.616 Iron | mdwild | 9.850 Hydrogen | Redock`.
- **Public extreme-rate log** (`/trading/information/history`, linked but not
  captured): records all trades that hit the rate extremes.

## Limits

From `/trading/information` → *My trading post* and the offer list:

- **Offers:** `x of 50` — account-wide cap **50** live offers.
- **Trading fleets:** `y of 200` — account-wide cap **200** concurrent trade
  deliveries.
- **Per-planet slots:** `12:147:1 = 8`, `12:147:3 = 0`, `12:147:4 = 0`. The
  populated planet has **Trading Post level 8** and the others have none —
  observed **slots = Trading Post level**. Unlocking trading on a planet
  therefore means building/upgrading its Trading Post.

## Marketplace list

LiveComponent `OfferOverview` props: `offered`, `requested`, `orderBy`,
`visibility`, `limit` (100). Filters: visibility (Public/Private), offered
resource, requested resource; sort `course_factor.asc|desc`,
`created.asc|desc`. Columns: **Offer | Rate | Demand | (Details)**.

Sample rows (the "Rate" is `course_factor = demand / offer`):

| Offer | Rate | Demand |
|---|---|---|
| 32.399.338 Iron | 0.302 | 9.784.600 Hydrogen |
| 44.423.792 Lutinum | 0.538 | 23.900.000 Hydrogen |
| 29.520.000 Lutinum | 2 | 59.040.000 Iron |
| 200.000 Iron | 98.364 | 19.672.800 Water |
| 960.000 Lutinum | 200.702 | 192.673.920 Water |

The offers above sit at the extreme ends of the allowed range (most are
"marked").

## Endpoint summary

| Method | Route | Purpose |
|---|---|---|
| GET | `/trading` | Marketplace (`OfferOverview`) |
| GET/POST | `/trading/create` (`?target=<uuid>`) | create public/private offer |
| GET/POST | `/trading/offer/<uuid>/detail` | view / accept |
| GET | `/trading/offer/<uuid>/revoke` | revoke (confirm) |
| GET | `/trading/own` | my live offers |
| GET | `/trading/own/history` | completed transactions |
| GET | `/trading/storage` | pending deliveries (+ Credit all) |
| GET | `/trading/information` | rate ratios, limits, fees |
| GET | `/trading/information/history` | public extreme-rate log |

## Relation to the catalog

The **Trading Post** building is already in `01-catalog.md` (base cost
`2.000 Iron, 2.000 Lutinum`, 10 pts/level, req Ship Factory 1). Its
technology-tree calculator exposes only cost/points/time — the **offer slots per
level** effect is not a calculator column; it is observed on
`/trading/information` (slots = Trading Post level, above).

## Differences vs gwx (what implementing this needs)

gwx has **no Trading Post and no trade subsystem at all**. A faithful port adds:

1. **Catalog + page:** a `trading_post` building (real cost/points from
   `01-catalog.md`), gating a `/trade` page; **slots = building level**.
2. **Tables:** `trade_offers` (uuid, planet, owner, offered ident+volume,
   requested ident, `course`, visibility, target player, `created_at`,
   `expires_at`, status) and `trade_transactions` (history rows) and
   `trade_storage` (pending deliveries with expiry).
3. **Universe config:** per-resource course values, max deviation (50 %), fee
   schedule, and durations — currently constants in the client.
4. **Delivery:** accepting spawns a **trading fleet** (travel time, the 200 cap)
   that moves goods into the recipient's trade storage; `Credit all` sweeps
   storage into planet stock on expiry/collection.
5. **Lifecycle:** create → partial/full accept → credit → history; revoke and
   expiry refund with fees retained.
6. **Marketplace list:** filters (visibility/offered/requested) + sorts
   (`rate`, `created`) over live public offers; the `OfferOverview` LiveComponent
   maps to a normal GET page (gwx is server-rendered, no LiveComponent).
