package digitalgoods

//go:generate cp --archive --dereference --target-directory ./html ../websites/digitalgoods.proxysto.re
//go:generate gotext-update-templates -srclang=en-US -lang=en-US,de-DE -out=catalog.go . ./cmd/digitalgoods ./html github.com/dys2p/eco/countries github.com/dys2p/eco/payment
