package main

import (
	"fmt"
	"net/http"
	"strings"
)

// AbsURL determines the absolute URL from an HTTP request.
//
// Make sure you have set "proxy_set_header Host $host;" besides proxy_pass in your nginx configuration.
func AbsHost(r *http.Request) string {
	var proto = "https"
	if strings.HasPrefix(r.Host, "127.0.") || strings.HasPrefix(r.Host, "[::1]") || strings.HasSuffix(r.Host, ".onion") { // if running locally or through TOR
		proto = "http"
	}
	return fmt.Sprintf("%s://%s", proto, r.Host)
}
