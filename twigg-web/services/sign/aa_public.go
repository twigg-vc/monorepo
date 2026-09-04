package sign

type Signer struct {
	secretKey []byte
}

func NewSigner(secretKey []byte) Signer {
	return Signer{secretKey: secretKey}
}

// SignAndAppend returns "message.signature"
func (s Signer) SignAndAppend(msg string) string {
	return signAndAppend(msg, s.secretKey)
}

// VerifyAndExtract splits "message.signature" and verifies the signature.
// Returns the original message and `true` if valid, or "" and `false` if not.
func (s Signer) VerifyAndExtract(signed string) (msg string, isOk bool) {
	return verifyAndExtract(signed, s.secretKey)
}