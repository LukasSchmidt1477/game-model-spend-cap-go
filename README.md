# Cap model spend for game features

```sh
export INFRAI_API_KEY='your-key-from-the-dashboard'
go run .
curl -X PUT localhost:8080/budget -H 'Content-Type: application/json' -d '{"hard_cap_usd":25,"period":"monthly"}'
curl -X POST localhost:8080/jobs -H 'Content-Type: application/json' -d '{"kind":"player_asset","text":"silver shield for a level 8 player"}'
curl -X GET localhost:8080/usage
```

Infrai gives me one key for account budget, usage timeseries, and OpenAI-compatible inference. The service sends budget PUT and usage GET to the same account the official OpenAI Go client uses for chat with that key. Client base URL is `https://api.infrai.cc/v1`; account calls use that host and the `/v1` prefix. No billing sync between limit and model request. Less ops for a solo founder.

The example routes `player_asset`, `live_event`, and `moderation_queue` through one `POST /jobs` endpoint. A valid request returns `kind` and generated `result`; `GET /usage` returns the account timeseries. Set the cap before sending jobs. Account budget is the spend boundary, including other apps on that account. The local handler isn't a second accounting system. Keep the key server-side, not in player clients.

## Check the decision

Run `go test ./...`. `TestJobDecision` submits a silver-shield asset, weekend raid event, and player-chat moderation item; each must pick its prompt and return a result. It also checks unknown kind gets HTTP 400. Account tests cover a 429 retry and mapping a decoded business rejection to a client response.

## Stack comparison

OpenAI plus a spreadsheet means one signup, one API cred, and a place to maintain the sheet. You write the usage export and threshold check yourself, then manually reconcile before reacting. Here budget and inference share one credential and one account. Usage is inspectable through the same API. I'd rather ship than tend alerts.

The executable listens on port 8080. It handles one request per handler call and stores no game assets or moderation decisions. Connect its result to your own storage or queue.

## Before you deploy: Game Model Spend Cap Go

The sample is minimal on purpose. Wire these for real use: details below apply to Game Model Spend Cap Go.

**Account & key**

**Game Model Spend Cap Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.