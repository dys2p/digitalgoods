# digitalgoods

We sell coupons using our BTCPay Server. No reservations are made (first pay, first serve). Underfulfilled purchases are fulfilled when goods are in stock again. digitalgoods uses an SQLite Database, continuous replication using [Litestream](https://litestream.io) is recommended.

## BTCPay Server Configuration

* User API Keys: enable `btcpay.store.canviewinvoices` and `btcpay.store.cancreateinvoice`
* Store Webhook
  * Payload URL: `https://example.com/rpc`
  * Automatic redelivery: yes
  * Is enabled: yes
  * Events: "An invoice is processing", "An invoice has expired", "An invoice has been settled"
