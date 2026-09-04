package digitalocean

import "encoding/base64"

const doRegion = "yes" // must be provided

func encodedObjectKey(keyPrefix, key string) string {
	s := base64.RawURLEncoding.EncodeToString([]byte(keyPrefix + "/" + key))
	return s
}