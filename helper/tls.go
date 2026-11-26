package helper

import "crypto/tls"

// RedisTLSConfig returns a TLS 1.2+ config unless envKey is set to false, for
// Redis servers without TLS (local development).
func RedisTLSConfig(envKey string) *tls.Config {
	if !ToBool(Env(envKey), true) {
		return nil
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}
}
