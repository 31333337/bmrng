package token

import (
	"github.com/31333337/bmrng/go/trellis/config"
	"github.com/31333337/bmrng/go/trellis/crypto/pairing/mcl"
)

// a fixed secret, public key pair to generate messages quickly
// to test other parts of the system
var SecretKey *TokenSigningKey
var PublicKey *TokenPublicKey

func init() {
	secret := &mcl.Fr{}
	secret.InterpretFrom(config.Seed[:mcl.FR_LEN])
	SecretKey = NewTokenSigningKey(secret)
	PublicKey = NewTokenPublicKey(&SecretKey.X)
}

func SkipToken(message []byte) *SignedToken {
	var hash mcl.G1
	t := &SignedToken{}
	PublicKey.hashToCurvePoint(message, &hash)
	SecretKey.BlindSign(&t.T, &hash)
	return t
}
