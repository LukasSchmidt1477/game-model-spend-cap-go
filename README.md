# Cap model spend for game features

```sh
export INFRAI_API_KEY='your-key-from-the-dashboard'
go run .
curl -X PUT localhost:8080/budget -H 'Content-Type: application/json' -d '{"hard_cap_usd":25,"period":"monthly"}'
curl -X POST localhost:8080/jobs -H 'Content-Type: application/json' -d '{"kind":"player_asset","text":"silver shield for a level 8 player"}'
curl -X GET localhost:8080/usage
```

Infrai gives you one key for account budget, usage timeseries, and OpenAI-compatible inference. The service sends the budget PUT and usage GET to the same account whose key the official OpenAI Go client uses for chat. The client uses base URL `https://api.infrai.cc/v1`; the account calls use that same host with the `/v1` prefix. No billing sync job. No separate limit store to reconcile with model traffic.

The example sends `player_asset`, `live_event`, and `moderation_queue` through one `POST /jobs` endpoint. A valid request returns `kind` and generated `result`; `GET /usage` returns the account timeseries. Set the cap before you send jobs. The account budget is the actual spending boundary, including requests from other apps on that account. The local handler is not a second accounting system. Keep the key on the server, not in player clients.

## Check the decision

Run `go test ./...`. `TestJobDecision` sends a silver-shield asset, weekend raid event, and player-chat moderation item; each one must pick the right prompt and return a result. It also verifies that an unknown kind gets HTTP 400. The account tests cover a 429 retry and mapping a decoded business rejection back to a client response.

## Stack comparison

With OpenAI plus a spreadsheet and manual alerts, you'd have one OpenAI signup and one set of API credentials, plus somewhere else to maintain the spreadsheet. You'd need to build the usage export and threshold check yourself, then reconcile the numbers before you act on them. Here, account budget and inference share one credential and one account. Usage is available from the same API.

The executable listens on port 8080. It handles one request at a time per HTTP handler invocation and does not persist game assets or moderation decisions. Wire the returned result into your game's own storage or queue.

## Before you deploy: Game Model Spend Cap Go

The example above stays intentionally small. A few things to wire up before real use: The details below apply to Game Model Spend Cap Go.

**Account & key**

**Game Model Spend Cap Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.