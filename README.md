# digitalgoods

We sell coupons using our BTCPay Server. No reservations are made (first pay, first serve). Underfulfilled purchases are fulfilled when goods are in stock again. digitalgoods uses an SQLite Database, continuous replication using [Litestream](https://litestream.io) is recommended.

## A short note on the security model

* Every purchase has a short but unique _ID_.
* The _access key_ gives access to the purchase. It is part of the purchase URL: `https://example.com/order/id/access_key`. It is important to use `rel="noreferrer"` in all outgoing links, so the URL won't leak.
* The purchase id and access key are stored in a cookie for eight hours. If a customer closes their purchase while paying through the BTCPay server, it can be restored without the BTCPay server knowing it.
* Every purchase has a _payment key_ as a safeguard for some payment methods.

## BTCPay Server Configuration

* User API Keys: enable `btcpay.store.canviewinvoices` and `btcpay.store.cancreateinvoice`
* Store Webhook
  * Payload URL: `https://example.com/rpc`
  * Automatic redelivery: yes
  * Is enabled: yes
  * Events: "An invoice is processing", "An invoice has expired", "An invoice has been settled"
