// Package static contains assets for the customer http router and the staff http router.
package static

import "embed"

//go:embed *
var Files embed.FS
