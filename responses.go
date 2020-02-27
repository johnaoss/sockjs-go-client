package sockjsclient

// Info represents the info received from a SockJS connection.
// From the spec, it is the result of calling `/info` on a
// SockJS server, as defined within the SockJS Spec
//
// Relevant link: http://sockjs.github.io/sockjs-protocol/sockjs-protocol-0.3.3.html
type Info struct {
	// WebSocket returns whether or not WebSockets are enabled on the server.
	WebSocket bool `json:"websocket"`
	// CookieNeeded determines if the transports need to support cookies or not
	// for load balancing purposes.
	CookieNeeded bool `json:"cookie_needed"`
	// Origins represents the list of allowed origins. It is currently ignored
	// as per the spec.
	Origins []string `json:"origins"`
	// Entropy is a "good, unpredicable random number from the
	// range [0, 2^31-1]" as defined by the spec
	Entropy int `json:"entropy"`
}
