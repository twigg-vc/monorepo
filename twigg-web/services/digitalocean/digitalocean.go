package digitalocean

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type spacesClient struct {
	endpoint   string
	folder     string
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

func newClient(bucketEndpoint, folder, accessKey, secretKey string) spacesClient {
	return spacesClient{
		endpoint:   strings.TrimRight(bucketEndpoint, "/"),
		folder:     folder,
		accessKey:  accessKey,
		secretKey:  secretKey,
		httpClient: &http.Client{},
	}
}

func (c spacesClient) Put(keyPrefix, key string, size int64, r io.Reader) error {
	url := fmt.Sprintf("%s/%s/%s", c.endpoint, c.folder, encodedObjectKey(keyPrefix, key))
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodPut, url, r)
	if err != nil {
		return err
	}
	req.ContentLength = size
	signRequest(req, c.accessKey, c.secretKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spaces put: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (c spacesClient) Get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error) {
	url := fmt.Sprintf("%s/%s/%s", c.endpoint, c.folder, encodedObjectKey(keyPrefix, key))
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, url, nil)
	if err != nil {
		return nil, func() {}, err
	}
	if offset > 0 {
		req.Header.Set("range", fmt.Sprintf("bytes=%d-", offset))
	}
	signRequest(req, c.accessKey, c.secretKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, func() {}, err
	}
	// A ranged read must be answered with 206; a 200 would mean the server
	// ignored the Range header and is sending the whole object.
	var wantStatus int
	if offset > 0 {
		wantStatus = http.StatusPartialContent
	} else {
		wantStatus = http.StatusOK
	}
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, func() {}, fmt.Errorf("spaces get: status %d: %s", resp.StatusCode, body)
	}
	return resp.Body, func() { _ = resp.Body.Close() }, nil
}

// signRequest adds AWS Signature V4 auth headers to req, following
// https://docs.digitalocean.com/reference/api/spaces/#authentication
func signRequest(req *http.Request, accessKey, secretKey string) {
	// Set the x-amz-date header:
	// x-amz-date	ISO 8601–formatted timestamp, for example 20170803T172753Z.
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	req.Header.Set("x-amz-date", amzDate)

	// Set x-amz-content-sha256:
	// x-amz-content-sha256	SHA256 hash of the request payload. Required for SigV4 authentication.
	payloadHash := "UNSIGNED-PAYLOAD" // UNSIGNED-PAYLOAD can be used when the hash is not computed (see s3 specs)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	// Construct canonicalHeaders variable.
	// Its a new-line-separated list of `header:header-val`
	host := req.URL.Host
	canonicalHeaders := "host:" + host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	// Create the canincalRequest:
	// canonicalRequest = `
	// ${HTTPMethod}\n
	// ${canonicalURI}\n
	// ${canonicalQueryString}\n
	// ${canonicalHeaders}\n
	// ${signedHeaders}\n
	// ${hashedPayload}
	// `
	canonicalRequest := req.Method + "\n" +
		req.URL.EscapedPath() + "\n" +
		req.URL.RawQuery + "\n" +
		canonicalHeaders + "\n" +
		signedHeaders + "\n" +
		payloadHash

	// Create the stringToSign:
	// stringToSign = "AWS4-HMAC-SHA256" + "\n" +
	//     date(format=ISO8601) + "\n" +
	//     date(format=YYYYMMDD) + "/" + ${REGION} + "/" + "s3/aws4_request" + "\n" +
	//     Hex(SHA256Hash(canonicalRequest))
	credentialScope := dateStamp + "/" + doRegion + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" +
		amzDate + "\n" +
		credentialScope + "\n" +
		hexSHA256(canonicalRequest)

	// Compute the signing key and use it to sing.
	signingKey := s3SigningKey(secretKey, dateStamp)
	// From the docs:
	// signature = Hex(HMAC-SHA256(signingKey, stringToSign))
	signature := fmt.Sprintf("%x", hmacSHA256(signingKey, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s,SignedHeaders=%s,Signature=%s",
		accessKey, credentialScope, signedHeaders, signature))
}

func s3SigningKey(secretKey, dateStamp string) []byte {
	// From the docs:
	// dateKey = HMAC-SHA256("AWS4" + ${SECRET_KEY}, date(format=YYYYMMDD))
	// dateRegionKey = HMAC-SHA256(dateKey, ${REGION})
	// dateRegionServiceKey = HMAC-SHA256(dateRegionKey, "s3")
	// signingKey = HMAC-SHA256(dateRegionServiceKey, "aws4_request")
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(doRegion))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
