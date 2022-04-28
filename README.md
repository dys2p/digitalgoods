# digitalgoods

We sell coupons using our BTCPay Server. No reservations are made (first pay, first serve). Underfulfilled purchases are fulfilled when goods are in stock again. digitalgoods uses an SQLite Database, continuous replication using [Litestream](https://litestream.io) is recommended.

## A short note on the security model

* The _purchase ID_ gives access to the purchase. It is part of the purchase URL: `https://example.com/i/purchase-id`. It is important to use `rel="noreferrer"` in all outgoing links, so the URL won't leak.
* Every purchase has a short but unique _pay ID_, which is used in payments. Payment processing does not know or require the purchase ID.
* The purchase ID is stored in a cookie for eight hours. If a customer closes their purchase while paying through the BTCPay server, it can be restored without the BTCPay server knowing it.

## BTCPay Server Configuration

* User API Keys: enable `btcpay.store.canviewinvoices` and `btcpay.store.cancreateinvoice`
* Store Webhook
  * Payload URL: `https://example.com/rpc`
  * Automatic redelivery: yes
  * Is enabled: yes
  * Events: "An invoice is processing", "An invoice has expired", "An invoice has been settled"
