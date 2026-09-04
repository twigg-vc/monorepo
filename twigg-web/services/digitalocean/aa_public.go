package digitalocean

import "io"

// MUST BE INITIALIZED WITH NewSpacesClient
type SpacesClient struct {
	s spacesClient
}

// All the data will be saved inside "folder"
func NewSpacesClient(bucketEndpoint, folder, accessKey, secretKey string) SpacesClient {
	return SpacesClient{newClient(bucketEndpoint, folder, accessKey, secretKey)}
}

func (s SpacesClient) Put(keyPrefix, key string, size int64, r io.Reader) error {
	return s.s.Put(keyPrefix, key, size, r)
}
func (s SpacesClient) Get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error) {
	return s.s.Get(keyPrefix, key, offset)
}
