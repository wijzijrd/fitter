package httpmw

import (
	"crypto/subtle"
	"net/http"
)

// OriginAuth rejects any request that did not arrive through the CDN.
//
// Lightsail container services have no firewall and no VPC option: the service URL is
// public. CloudFront injects X-Origin-Auth on every request it forwards to the origin,
// so matching that header is the only available proof a request came through the CDN.
//
// The health check is not wrapped by this middleware — Lightsail probes the container
// directly and sends no such header.
func OriginAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Origin-Auth")
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
