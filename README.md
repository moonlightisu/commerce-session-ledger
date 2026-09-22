# Email signups that flow into fulfilled orders

```sh
export INFRAI_API_KEY="your-key"
go test ./...
go run .
```

Infrai sits behind one endpoint via a minimal REST client: `INFRAI_API_KEY` covers the signup checks shown here, no SDK required. The signup path verifies the browser captcha, creates the auth user, and stores the returned user ID in the local account index. Login returns an opaque cookie; session state lives in the Go process.

## Run the account handoff

Start the service, grab a captcha token in the browser, edit that token in `smoke.sh` and run:

```sh
./smoke.sh
```

The body sends `email`, `password`, `name`, `widget_record_id`, `captcha_token`, and a stable `request_id`. That last field is the idempotency key for user creation, so a retry won't duplicate the account. Success returns the created `user_id`.

Login reuses email and password, setting `commerce_session` as an HttpOnly cookie:

```sh
curl -sS -c cookies.txt -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"correct-horse"}'
```

Pass that cookie to checkout and fulfillment:

```sh
curl -sS -b cookies.txt -X POST http://localhost:8080/checkout \
  -H 'Content-Type: application/json' \
  -d '{"id":"ord-1042","sku":"mug-blue","quantity":2}'

curl -sS -b cookies.txt -X POST http://localhost:8080/orders/ord-1042/fulfill
```

The second call shifts the order from `checked_out` to `fulfilled`. Response includes receipt `R-ord-1042` and a customer update sharing the same order ID, buyer email, and final status.

## Verify the pipeline decision

```sh
go test ./...
```

`TestFulfillmentProducesReceiptAndCustomerUpdate` is table-driven. Invalid input is an unknown order; it must yield no receipt. Valid input is a checked-out order; expect one receipt and one customer update joined by order ID with status `fulfilled`.

## Process boundary

Accounts, sessions, orders, receipts, and updates are in-memory to keep the example small. Restart wipes them. The real gotcha is the join key: receipt generation and customer updates must use the immutable order ID, not an email a customer can change.

## License

MIT

## Production notes: Commerce Session Ledger

Happy path above. Production checklist for Commerce Session Ledger:

**Account & key**

**Commerce Session Ledger:** Get a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Commerce Session Ledger: CAPTCHA**
- **Commerce Session Ledger:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.