package srvconfig

import (
	"bytes"
	"fmt"
	"monorepo/base/serverutils"
	"monorepo/twigg-web/server/seed"
	"monorepo/twigg-web/services/secrets"
	"monorepo/twigg-web/services/user"
	"os"
	"time"
)

// Struct that configures the server
type SrvConfig struct {
	// Name of the config
	Name string
	// Server full public URL. Examples: https://twigg.vc, http://localhost:9000
	PublicUrl string
	// Port the server listens to
	Port int
	// Absolute path to a directory under which all data is stored
	StorageFolderAbsPath string
	// Size of the blocks used to store blobs.
	// Recommended for prod is >1GB.
	StorageBlockSize int64
	// Number of blobs that are cached to disk. Must be >=1.
	BlobStorageCacheCapacity int

	// Full url (e.g. `https://track.twigg.vc``) to the track server
	// - the server to which the jobs will be posted. It might be "" - which
	// indicates nothing should be posted to it.
	TrackServerUrl string
	// Key used to authenticate to the track server
	TrackServerKey string

	// Api key used to authenticate with this server to post CICD webhooks.
	// All requests must have the `TwiggServerKey` header with this value
	TwiggServerKey string

	// Key used to sign and verify twigg tokens
	TwiggTokenSigningKey []byte

	// If set, the blobs will spill to digital ocean object storage (spaces).
	// This requires setting the bucket name and keys
	UseDigitalOceanSpaces             bool
	DigitalOceanSpacesBucketUrl       string
	DigitalOceanSpacesAccessKeyId     string
	DigitalOceanSpacesAccessKeySecret string
	// All data will be saved under this folder of the bucket
	DigitalOceanSpacesFolderName string

	// Max "queries" (requests) per second for the rate limiter
	RateLimitMaxQps float64
	// Max burst for rate limiter
	RateLimitMaxQpsBurst int

	// Specifies how long the queue runner sleeps waiting for new messages
	QueueRunnerSleep time.Duration
	// Specifies how many queue tasks can run in parallel
	QueueConcurrency int

	// If set, db blob quota usage will not be enforced
	DisableQuotaEnforcement bool

	// If set, authentication will be mocked to be authenticated of user Aang.
	// Auth cookies are not used.
	MockAuthUser bool

	// If set, the insecure (non-https) cookies will be sent on authentication
	// responses. This only applies if MockAuthUser is false.
	InsecureAuthCookies bool

	// If set, the server won't check for CSRF headers on every non-GET request.
	// This only applies if MockAuthUser is false.
	DisableStrongCsrfProtection bool

	// Which stripe will use (mock, prod...)
	StripeMode StripeMode
	// Used for stripe modes that require a key
	StripeSecretKey string
	// Used for stripe modes that require endpoint secret
	StripeEndpointSecret string

	// If set, will register a handler at GET /fake-oauth that returns "ok"
	RegisterFakeOauthHandler bool

	// It set, the mock GoogleOAuth client will be used.
	// This should only be used in tests so we can mock google oauth responses.
	UseGoogleOAuthClientMock bool
	// Used for Google Oauth mode that required google client id
	GoogleClientId string
	// Used for Google Oauth mode that required google client secret
	GoogleClientSecret string

	// Used for Microsoft Azure Oauth mode that required client id
	MsAzureOAuthClientId string
	// Used for Microsoft Azure mode that required client secret
	MsAzureOAuthClientSecret string

	// If set, will use the mock for the keys generation
	UseKeysMock bool

	// If set, users can login with email and password
	AllowPasswordLogin bool

	// If set, db migrations won't be run
	SkipMigrations bool

	// HasSecretsMasterKey SHOULD NOT BE USED IN PRODUCTION.
	// If true, SecretsMasterKey is used as secret's master key.
	// Else, a dummy value is used.
	HasSecretsMasterKey bool
	// 32 byte, base64-encoded value used for the secrets master key if HasSecretsMasterKey=true
	SecretsMasterKey string

	// Salt used when hashing passwords and CLI keys
	PasswordSalt string

	// Emails that can access the admin dashboard
	AdminEmails []string

	// If set, the server's db will be populated on startup with
	// SeedUsers if they don'y already exist.
	SeedUsers []seed.SeedUser
	// If set, the server's db will be populated on startup with
	// SeedRepos if they don'y already exist.
	SeedRepos []seed.SeedRepo
}

func (c SrvConfig) MasterKey() []byte {
	if !c.HasSecretsMasterKey {
		return bytes.Repeat([]byte{1}, 32)
	}
	key, err := secrets.ParseMasterKey(c.SecretsMasterKey)
	if err != nil {
		panic(fmt.Sprintf("failed to read master key from env: %s", err))
	}
	return key
}

// Returns the env var value, or a fallback if the env value is empty
func GetEnvOr(envVarName, fallback string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		return fallback
	}
	return value
}

// Returns the env var value, or panics if it's unset or empty.
func GetEnvOrDie(envVarName string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		panic(fmt.Sprintf("environment variable %q is required but not set", envVarName))
	}
	return value
}

// Creates necessary folders to use the StorageFolderAbsPath.
// Always call this before using c.StorageFolderAbsPath
func (c *SrvConfig) AdjustStorageFolderAbsPath() {
	serverutils.AdjustServerDirOrDie(&c.StorageFolderAbsPath, "twiggData")
}

// Returns the config used for testing
func MockConfig(Port int,
	StorageFolderAbsPath string,
	TwiggServerKey string, TrackServerUrl, TrackServerKey string) SrvConfig {
	return SrvConfig{
		Name:                        "mock",
		PublicUrl:                   fmt.Sprintf("http://localhost:%d", Port),
		Port:                        Port,
		StorageFolderAbsPath:        StorageFolderAbsPath,
		StorageBlockSize:            1024, // 1 kB
		BlobStorageCacheCapacity:    1,
		TrackServerUrl:              TrackServerUrl,
		TrackServerKey:              TrackServerKey,
		TwiggServerKey:              TwiggServerKey,
		TwiggTokenSigningKey:        []byte("mock-token-sig-key"),
		RateLimitMaxQps:             200,
		RateLimitMaxQpsBurst:        100,
		DisableQuotaEnforcement:     false,
		InsecureAuthCookies:         true,
		DisableStrongCsrfProtection: true,
		StripeMode:                  StripeMode_mock,
		RegisterFakeOauthHandler:    true,
		UseGoogleOAuthClientMock:    true,
		UseKeysMock:                 true,
		AllowPasswordLogin:          true,
		SkipMigrations:              true,
		PasswordSalt:                "salty",
		QueueRunnerSleep:            300 * time.Millisecond,
		QueueConcurrency:            5,
		AdminEmails:                 []string{"aang@twigg.vc"},
		SeedUsers: []seed.SeedUser{
			{
				Username: "aang",
				Email:    "aang@twigg.vc",
				Password: "yipyip",
				Sub:      user.Subscription_Team, SubQuantity: 4,
			},
			{
				Username: "katara",
				Email:    "katara@twigg.vc",
				Password: "fullmoon",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
			{
				Username: "sokka",
				Email:    "sokka@twigg.vc",
				Password: "suki4ever",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
			{
				Username: "toph",
				Email:    "toph@twigg.vc",
				Password: "badgermole",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
		},
		SeedRepos: []seed.SeedRepo{
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookOne",
				RepoDescription:        "Water",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookTwo",
				RepoDescription:        "Earth",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookThree",
				RepoDescription:        "Fire",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
		},
	}
}

// Returns a config for local development
func LocalConfig(Port int,
	StorageFolderAbsPath string,
	TwiggServerKey, TrackServerUrl, TrackServerKey string) SrvConfig {
	return SrvConfig{
		Name:                     "local",
		PublicUrl:                fmt.Sprintf("http://localhost:%d", Port),
		Port:                     Port,
		StorageFolderAbsPath:     StorageFolderAbsPath,
		StorageBlockSize:         4 * 1024 * 1024 * 1024, // 4 GB
		BlobStorageCacheCapacity: 1,
		TrackServerUrl:           TrackServerUrl,
		TrackServerKey:           TrackServerKey,
		TwiggServerKey:           TwiggServerKey,
		TwiggTokenSigningKey:     []byte("mock-token-sig-key"),
		RateLimitMaxQps:          200,
		RateLimitMaxQpsBurst:     100,
		DisableQuotaEnforcement:  false,
		MockAuthUser:             true,
		StripeMode:               StripeMode_test,
		StripeSecretKey:          GetEnvOr("TWIGG_STRIPE_SECRET_KEY", ""),
		StripeEndpointSecret:     GetEnvOr("TWIGG_STRIPE_ENDPOINT_SECRET", ""),
		GoogleClientId:           GetEnvOr("TWIGG_GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:       GetEnvOr("TWIGG_GOOGLE_CLIENT_SECRET", ""),
		MsAzureOAuthClientId:     GetEnvOr("TWIGG_MS_AZURE_CLIENT_ID", ""),
		MsAzureOAuthClientSecret: GetEnvOr("TWIGG_MS_AZURE_CLIENT_SECRET", ""),
		AllowPasswordLogin:       true,
		PasswordSalt:             "salty",
		QueueRunnerSleep:         10 * time.Second,
		QueueConcurrency:         5,
		AdminEmails:              []string{"aang@twigg.vc"},
		SeedUsers: []seed.SeedUser{
			{
				Username: "aang",
				Email:    "aang@twigg.vc",
				Password: "yipyip",
				Sub:      user.Subscription_Team, SubQuantity: 4,
			},
			{
				Username: "katara",
				Email:    "katara@twigg.vc",
				Password: "fullmoon",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
			{
				Username: "sokka",
				Email:    "sokka@twigg.vc",
				Password: "suki4ever",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
			{
				Username: "toph",
				Email:    "toph@twigg.vc",
				Password: "badgermole",
				Sub:      user.Subscription_Trial, SubQuantity: 1,
			},
		},
		SeedRepos: []seed.SeedRepo{
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookOne",
				RepoDescription:        "Water",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookTwo",
				RepoDescription:        "Earth",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
			{
				RepoOwnerUsername:      "aang",
				RepoName:               "BookThree",
				RepoDescription:        "Fire",
				UsernamesWithWritePerm: []string{"sokka", "katara", "toph"},
			},
		},
	}
}

// Returns the config that runs on the homelab
func HomelabConfig(Port int,
	StorageFolderAbsPath string,
	TrackServerUrl string) SrvConfig {
	return SrvConfig{
		Name:                              "lab",
		PublicUrl:                         "https://homelab.twigg.vc",
		Port:                              Port,
		StorageBlockSize:                  256 * 1024 * 1024, // 256 MB
		BlobStorageCacheCapacity:          5,
		TrackServerUrl:                    TrackServerUrl,
		TrackServerKey:                    GetEnvOrDie("TWIGG_TRACK_KEY"),
		TwiggServerKey:                    GetEnvOrDie("TWIGG_SERVER_KEY"),
		TwiggTokenSigningKey:              []byte(GetEnvOrDie("TWIGG_TOKEN_SIGNING_KEY")),
		UseDigitalOceanSpaces:             true,
		DigitalOceanSpacesBucketUrl:       "https://twigg-lab.nyc3.digitaloceanspaces.com",
		DigitalOceanSpacesAccessKeyId:     GetEnvOrDie("TWIGG_DO_SPACES_ACCESS_KEY_ID"),
		DigitalOceanSpacesAccessKeySecret: GetEnvOrDie("TWIGG_DO_SPACES_ACCESS_KEY_SECRET"),
		DigitalOceanSpacesFolderName:      "lab_256",
		RateLimitMaxQps:                   200,
		RateLimitMaxQpsBurst:              100,
		StorageFolderAbsPath:              StorageFolderAbsPath,
		StripeMode:                        StripeMode_test,
		StripeSecretKey:                   GetEnvOrDie("TWIGG_STRIPE_SECRET_KEY"),
		StripeEndpointSecret:              GetEnvOrDie("TWIGG_STRIPE_ENDPOINT_SECRET"),
		GoogleClientId:                    GetEnvOrDie("TWIGG_GOOGLE_CLIENT_ID"),
		GoogleClientSecret:                GetEnvOrDie("TWIGG_GOOGLE_CLIENT_SECRET"),
		MsAzureOAuthClientId:              GetEnvOrDie("TWIGG_MS_AZURE_CLIENT_ID"),
		MsAzureOAuthClientSecret:          GetEnvOrDie("TWIGG_MS_AZURE_CLIENT_SECRET"),
		AllowPasswordLogin:                false,
		HasSecretsMasterKey:               true,
		SecretsMasterKey:                  GetEnvOrDie("TWIGG_MASTER_KEY"),
		PasswordSalt:                      GetEnvOrDie("TWIGG_PASSWORD_SALT"),
		QueueRunnerSleep:                  10 * time.Second,
		QueueConcurrency:                  1,
		AdminEmails: []string{
			"andre@twigg.vc", "joao@twigg.vc", "marcos@twigg.vc"},
	}
}

// Returns the config used for production
func ProdConfig(Port int,
	StorageFolderAbsPath string,
	TrackServerUrl string) SrvConfig {
	return SrvConfig{
		Name:                              "prod",
		PublicUrl:                         "https://twigg.vc",
		Port:                              Port,
		StorageFolderAbsPath:              StorageFolderAbsPath,
		StorageBlockSize:                  256 * 1024 * 1024, // 256 MB
		BlobStorageCacheCapacity:          10,
		TrackServerUrl:                    TrackServerUrl,
		TrackServerKey:                    GetEnvOrDie("TWIGG_TRACK_KEY"),
		TwiggServerKey:                    GetEnvOrDie("TWIGG_SERVER_KEY"),
		TwiggTokenSigningKey:              []byte(GetEnvOrDie("TWIGG_TOKEN_SIGNING_KEY")),
		UseDigitalOceanSpaces:             true,
		DigitalOceanSpacesBucketUrl:       "https://twigg-bucket.nyc3.digitaloceanspaces.com",
		DigitalOceanSpacesAccessKeyId:     GetEnvOrDie("TWIGG_DO_SPACES_ACCESS_KEY_ID"),
		DigitalOceanSpacesAccessKeySecret: GetEnvOrDie("TWIGG_DO_SPACES_ACCESS_KEY_SECRET"),
		DigitalOceanSpacesFolderName:      "prod_256",
		RateLimitMaxQps:                   200,
		RateLimitMaxQpsBurst:              100,
		StripeMode:                        StripeMode_prod,
		StripeSecretKey:                   GetEnvOrDie("TWIGG_STRIPE_SECRET_KEY"),
		StripeEndpointSecret:              GetEnvOrDie("TWIGG_STRIPE_ENDPOINT_SECRET"),
		GoogleClientId:                    GetEnvOrDie("TWIGG_GOOGLE_CLIENT_ID"),
		GoogleClientSecret:                GetEnvOrDie("TWIGG_GOOGLE_CLIENT_SECRET"),
		MsAzureOAuthClientId:              GetEnvOrDie("TWIGG_MS_AZURE_CLIENT_ID"),
		MsAzureOAuthClientSecret:          GetEnvOrDie("TWIGG_MS_AZURE_CLIENT_SECRET"),
		AllowPasswordLogin:                false,
		HasSecretsMasterKey:               true,
		SecretsMasterKey:                  GetEnvOrDie("TWIGG_MASTER_KEY"),
		PasswordSalt:                      GetEnvOrDie("TWIGG_PASSWORD_SALT"),
		QueueRunnerSleep:                  10 * time.Second,
		QueueConcurrency:                  1,
		AdminEmails: []string{
			"andre@twigg.vc", "joao@twigg.vc", "marcos@twigg.vc"},
	}
}

type StripeMode int

const (
	StripeMode_empty StripeMode = iota
	StripeMode_mock
	StripeMode_test
	StripeMode_prod
)
