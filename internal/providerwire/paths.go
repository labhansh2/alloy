// Package providerwire defines internal native-provider gateway paths.
package providerwire

const Prefix = "/alloy/providers/"

func AccessPath(name string) string {
	return Prefix + name
}

func GatewayPath(name string) string {
	return AccessPath(name) + "/http"
}
