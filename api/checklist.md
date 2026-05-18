# Alebus API Checklist (v1)

This checklist is a planning aid for implementation + clients.

Legend:
- **Role**: Public (commuter), Operator, Admin
- **Auth**: placeholder (not implemented yet); keep role separation in mind.

## Public (Commuter)

- [ ] `GET /api/v1/health` — Public — Auth: none
- [ ] `GET /health` — Public — Auth: none (legacy convenience)

- [ ] `GET /api/v1/routes` — Public — Auth: none
- [ ] `GET /api/v1/stops/nearby` — Public — Auth: none

- [ ] `GET /api/v1/journeys` — Public — Auth: user
- [ ] `GET /api/v1/journeys/smart-plan` — Public — Auth: user/none (decide)
- [ ] `GET /api/v1/journeys/two-leg-plan` — Public — Auth: user/none (decide)
- [ ] `POST /api/v1/journeys/create` — Public — Auth: user
- [ ] `POST /api/v1/journeys/board` — Public — Auth: user
- [ ] `POST /api/v1/journeys/decline` — Public — Auth: user
- [ ] `POST /api/v1/journeys/complete` — Public — Auth: user
- [ ] `POST /api/v1/journeys/cancel` — Public — Auth: user

- [ ] `POST /api/v1/users/create` — Public — Auth: none
- [ ] `POST /api/v1/users/email` — Public — Auth: user
- [ ] `POST /api/v1/users/subscription` — Public — Auth: user/admin (decide)
- [ ] `POST /api/v1/users/location/add` — Public — Auth: user
- [ ] `POST /api/v1/users/location/remove` — Public — Auth: user

## Operator (Fleet + Monitoring)

- [ ] `GET /api/v1/buses` — Operator — Auth: operator
- [ ] `POST /api/v1/buses/direction` — Operator — Auth: operator
- [ ] `POST /api/v1/buses/status` — Operator — Auth: operator

- [ ] `GET /api/v1/redis/status` — Operator — Auth: operator
- [ ] `GET /api/v1/redis/buses` — Operator — Auth: operator

- [ ] `GET /api/v1/events` — Operator — Auth: operator

- [ ] `POST /api/v1/journeys/refresh` — Operator — Auth: operator
- [ ] `POST /api/v1/journeys/switch-by-location` — Operator — Auth: operator
- [ ] `GET /api/v1/journeys/fresh-recs-status` — Operator — Auth: operator

## Admin (Management + Debug)

- [ ] `GET /api/v1/db/status` — Admin — Auth: admin
- [ ] `POST /api/v1/db/reset` — Admin — Auth: admin

- [ ] `POST /api/v1/routes/create` — Admin — Auth: admin
- [ ] `POST /api/v1/routes/status` — Admin — Auth: admin
- [ ] `POST /api/v1/routes/sample` — Admin — Auth: admin
- [ ] `POST /api/v1/routes/recalculate` — Admin — Auth: admin

- [ ] `POST /api/v1/buses/create` — Admin — Auth: admin
- [ ] `POST /api/v1/buses/sample-raw` — Admin — Auth: admin
- [ ] `POST /api/v1/buses/simulate-gps` — Admin — Auth: admin
- [ ] `POST /api/v1/buses/simulate-all-gps` — Admin — Auth: admin

- [ ] `GET /api/v1/users` — Admin — Auth: admin
- [ ] `POST /api/v1/users/sample` — Admin — Auth: admin

- [ ] `POST /api/v1/journeys/sample` — Admin — Auth: admin
- [ ] `POST /api/v1/journeys/arrive` — Admin — Auth: admin
- [ ] `POST /api/v1/journeys/reset` — Admin — Auth: admin

- [ ] `GET /api/v1/debug/gps-enrichment` — Admin — Auth: admin
- [ ] `POST /api/v1/events/clear` — Admin — Auth: admin
- [ ] `POST /api/v1/redis/reset` — Admin — Auth: admin
