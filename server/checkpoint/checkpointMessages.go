package checkpoint

import (
	"https://github.com/31333337/trellis/crypto"
	"https://github.com/31333337/trellis/crypto/token"
)

type CheckpointInfo struct {
	// For simplicity we just send the signing key separately and don't worry about converting
	AnonymousVerificationKey crypto.VerificationKey
	Token                    token.SignedToken // signs round||layer||sending-server||PublicKey||SigningKey
}

type CheckpointResponse struct {
	PublicKey  crypto.LookupKey
	PartialKey crypto.DHPublicKey
}
