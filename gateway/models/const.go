package models

const (
	ClientIDHeaderKey      = "x-gateway-key"
	TimestampHeaderKey     = "x-gateway-timestamp"
	SignatureHeaderKey     = "x-gateway-signature"
	ContentTypeHeaderKey   = "content-type"
	AuthorizationHeaderKey = "authorization"
	UserAgentHeaderKey     = "user-agent"

	BvContentTypeHeaderKey     = "bv-content-type"
	BvRealIPTypeHeaderKey      = "bv-real-ip"
	BvRealIPProofTypeHeaderKey = "bv-real-ip-proof"
	BvXRemoteAddrTypeHeaderKey = "bv-x-remote-addr"

	ContentTypeValueJSON = "application/json"

	ClientInfoContextValueKey        = "client-info"
	ServiceContextValueKey           = "service"
	PathContextValueKey              = "path"
	AuthorizationInfoContextValueKey = "x-authorization-info"
)
