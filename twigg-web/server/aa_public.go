package server

import (
	"context"
	"fmt"
	"log"
	"monorepo/buildmeta"
	"monorepo/squeue"
	"monorepo/tw/bin"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/cicdqueue"
	"monorepo/twigg-web/docusaurus"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/handlers/admindash"
	"monorepo/twigg-web/handlers/commit"
	"monorepo/twigg-web/handlers/home"
	"monorepo/twigg-web/handlers/jobshandler"
	"monorepo/twigg-web/handlers/landing"
	"monorepo/twigg-web/handlers/login"
	"monorepo/twigg-web/handlers/needupgrade"
	"monorepo/twigg-web/handlers/newrepo"
	"monorepo/twigg-web/handlers/notifications"
	"monorepo/twigg-web/handlers/organization"
	"monorepo/twigg-web/handlers/payments"
	"monorepo/twigg-web/handlers/plans"
	"monorepo/twigg-web/handlers/reposettings"
	"monorepo/twigg-web/handlers/repository"
	"monorepo/twigg-web/handlers/termsandprivacy"
	"monorepo/twigg-web/handlers/track"
	"monorepo/twigg-web/handlers/twigg"
	"monorepo/twigg-web/handlers/usereducation"
	"monorepo/twigg-web/handlers/usersettings"
	"monorepo/twigg-web/handlers/welcome"
	"monorepo/twigg-web/metrics"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/server/seed"
	"monorepo/twigg-web/services/cansubcache"
	"monorepo/twigg-web/services/cicdparser"
	"monorepo/twigg-web/services/cicdpublisher"
	jobsservice "monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/services/keys"
	"monorepo/twigg-web/services/memlogger"
	"monorepo/twigg-web/services/mirror"
	"monorepo/twigg-web/services/oauthclient"
	"monorepo/twigg-web/services/orghelper"
	"monorepo/twigg-web/services/owners"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/review"
	"monorepo/twigg-web/services/secrets"
	"monorepo/twigg-web/services/sign"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/trackqueue"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/srvconfig"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/webcomponents/bundles"
	"monorepo/twigg-web/wrappers"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Main Twigg server that hosts repositories, documentation, code review
// yadayada (i.e. the actual twigg.vc). Must be initialized with NewSrv.
type Srv struct {
	C srvconfig.SrvConfig

	// IsReady is true when the server initialization is complete
	IsReady bool
	// StripeClientMock contains the mocked stripe client when IsReady=true
	// and the config has StripeMode=StripeMode_mock
	StripeClientMock stripeclient.StripeClientMock
	// QueueRunner contains the server queue after IsReady=true
	QueueRunner squeue.Runner
	// GoogleOAuthClientMock contains the mock Google OAuth client after
	// IsReady=true if the config specified UseGoogleOAuthClientMock=true
	GoogleOAuthClientMock oauthclient.GoogleMock
	// KeysMock contains a mock keys service after IsReady=true if
	// the config specified that UseKeysMock=true
	KeysMock keys.MockService

	mux           wrappers.RlMux
	trackObserver TrackObserver
}

// Initializes a ready-to-use sever instance
func NewSrv(cfg srvconfig.SrvConfig) Srv {
	return Srv{
		C: cfg,
	}
}

// Used to observe interactions with the track server
type TrackObserver interface {
	OnTrackWebhookReceived(job trackclient.TrackJob, payload runnerlib.JobPayload)
}

// Sets the TrackObserver so that webhooks can be observed
func (s *Srv) SetTrackObserver(obs TrackObserver) {
	s.trackObserver = obs
}

// Initializes all the dependencies and runs the http server.
// This method blocks until SIGINT/SIGTERM is received. It auto-cleans up then.
// If runInMaintenanceMode, serves just a very simple "maintenance" handler.
func (s *Srv) Run(runInMaintenanceMode bool) {
	log.Println("twigg-web server is starting...")
	if runInMaintenanceMode {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("maintenance"))
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, MaintenanceModeMsg)
			duration := time.Since(startTime)
			log.Printf("%s %s %s", r.Method, r.URL.Path, duration)
		})
		fmt.Printf("Serving maintenance on :%d\n", s.C.Port)
		err := http.ListenAndServe(":"+strconv.Itoa(s.C.Port), mux)
		if err != nil {
			fmt.Printf("server exited with err: %s\n", err)
		}
		return
	}
	s.C.AdjustStorageFolderAbsPath()
	if s.C.QueueRunnerSleep <= 0 {
		panic("got QueueRunnerSleep<=0")
	}
	if s.C.QueueConcurrency <= 0 {
		panic("got QueueConcurrency<=0")
	}
	sDb, closeSdb := getDb(s.C)
	rt := routes.New()
	queueStorage, closeQueueStorage, err := squeue.NewSqliteStorage(s.C.StorageFolderAbsPath)
	if err != nil {
		panic(fmt.Sprintf("failed to create queue service: %s", err))
	}
	s.QueueRunner = squeue.NewRunner(queueStorage, s.C.QueueRunnerSleep, s.C.QueueConcurrency)
	const metricsFlushInternal = 30 * time.Second // Sample metrics every 30 sec
	const cleanupIntervalInSeconds = 2 * 60 * 60  // cleanup every 2h
	const metricRetentionInSeconds = 24 * 60 * 60 // retain metrics for 24h
	mService, closeMetricsService, err := metrics.New(
		s.C.StorageFolderAbsPath, metricsFlushInternal,
		cleanupIntervalInSeconds, metricRetentionInSeconds)
	if err != nil {
		log.Fatalf("failed to get metrics service: %s", err)
		return
	}

	// Create Services
	const memLoggerInterval = 60 * time.Minute // Log memory use every 60 min
	memLogger := memlogger.New(memLoggerInterval, mService)

	masterKey := s.C.MasterKey()
	secretsSrv, err := secrets.NewService(sDb, masterKey)
	if err != nil {
		log.Fatalf("failed to setup secrets: %s", err)
		return
	}
	rSrv_, err := repo.NewService(sDb, secretsSrv)
	if err != nil {
		log.Fatalf("failed to setup repo: %s", err)
		return
	}
	rSrv := RepoServiceAdaptor{rSrv_}
	ownersSrv := owners.New(rSrv)

	// canSubCache is used to cache results from the handler that determines
	// if a commit can/can't be submitted. The cache auto-evicts entries after
	// it gets full. Larger canSubCacheSize will improve
	// the performance of this handler but consume more memory.
	const canSubCacheSize = 50_000
	canSubCache := cansubcache.New(canSubCacheSize)
	log.Printf(
		"CanSubCache will use ~%.1fMB memory",
		// Use a 2x factor bc its a rough estimate
		float64(canSubCache.GetMaxMemUsageEstimate()*2)/(1024*1024),
	)

	var stripeClient stripeclient.StripeClient
	switch s.C.StripeMode {
	case srvconfig.StripeMode_mock:
		s.StripeClientMock = stripeclient.NewMockStripeClient()
		stripeClient = s.StripeClientMock
	case srvconfig.StripeMode_test:
		stripeClient = stripeclient.NewTestStripeClient(s.C.PublicUrl, s.C.StripeSecretKey, s.C.StripeEndpointSecret)
	case srvconfig.StripeMode_prod:
		stripeClient = stripeclient.NewStripeClient(s.C.PublicUrl, s.C.StripeSecretKey, s.C.StripeEndpointSecret)
	default:
		panic("invalid stripe mode")
	}
	jobsService := jobsservice.NewService(sDb)
	ciCdFileParser := cicdparser.CiCdFileParser{}
	trackClient := trackclient.NewClient(s.C.TrackServerUrl, s.C.TrackServerKey)
	trackQueue, err := trackqueue.New(jobsService, trackClient, sDb)
	if err != nil {
		log.Fatalf("failed to setup trackqueue: %s", err)
		return
	}
	userSrv_, err := userservice.NewService(trackQueue, stripeClient, sDb, s.C.PasswordSalt)
	if err != nil {
		log.Fatalf("failed to setup users: %s", err)
		return
	}
	userSrv := UserServiceAdaptor{userSrv_}

	flagsHelper := featureflags.NewFlagsHelper(s.C.Name, userSrv)

	var keysService keys.Service
	if s.C.UseKeysMock {
		s.KeysMock = keys.NewMock()
		keysService = s.KeysMock
	} else {
		keysService = keys.New()
	}
	revSrv, err := review.New(sDb, ownersSrv, userSrv)
	if err != nil {
		log.Fatalf("failed to setup review: %s", err)
		return
	}
	tokenSigner := sign.NewSigner(s.C.TwiggTokenSigningKey)
	ciService := cicdpublisher.New(rSrv, userSrv, jobsService,
		ciCdFileParser, trackQueue, flagsHelper, tokenSigner)
	ciQueue, err := cicdqueue.New(jobsService, ciService, sDb, s.QueueRunner)
	if err != nil {
		log.Fatalf("failed to setup ci-queue service: %s", err)
		return
	}
	if !filepath.IsAbs(s.C.StorageFolderAbsPath) {
		log.Fatalf("s.c.StorageFolderAbsPath=%q in not abs", s.C.StorageFolderAbsPath)
		return
	}
	mirrorSrv, err := mirror.New(filepath.Join(s.C.StorageFolderAbsPath, "git-mirror-wd"))
	if err != nil {
		log.Fatalf("failed to setup git mirror service: %s", err)
		return
	}
	orgHelper := orghelper.NewHelper(sDb, getAllOrgRepoIdsAdaptor{repoService: rSrv})
	if !s.C.SkipMigrations {
		runGoMigrations(sDb)
	}

	// Seed the db with some users and repos if the config specifies
	if len(s.C.SeedUsers) != 0 {
		seed.CreateUsersIfNotExistOrDie(s.C.SeedUsers, sDb, userSrv)
	}
	if len(s.C.SeedRepos) != 0 {
		seed.CreateRepoIfNotExistsOrDie(s.C.SeedRepos, sDb, userSrv, rSrv)
	}

	var googleOAuthClient oauthclient.Google
	if s.C.UseGoogleOAuthClientMock {
		s.GoogleOAuthClientMock = oauthclient.NewGoogleMock()
		googleOAuthClient = s.GoogleOAuthClientMock
	} else {
		googleOAuthClient = oauthclient.NewLocalGoogle(s.C.PublicUrl, s.C.GoogleClientId, s.C.GoogleClientSecret)
	}
	microsoftOAuthClient := oauthclient.NewLocalMicrosoft(s.C.PublicUrl, s.C.MsAzureOAuthClientId, s.C.MsAzureOAuthClientSecret)
	sessionService := s.getSessionService(sDb, userSrv, googleOAuthClient, microsoftOAuthClient)

	// Create muxes
	nonRatelimittedMux := http.NewServeMux()
	s.mux = wrappers.NewRateLimitted(
		s.C.RateLimitMaxQps, s.C.RateLimitMaxQpsBurst,
		mService, sessionService, nonRatelimittedMux)
	authMux := wrappers.NewAuthMux(sessionService, s.C.Name, s.mux)
	userMux := wrappers.NewUserMux(authMux, stripeClient, sDb, userSrv)
	adminMux := wrappers.NewAdminUserMux(userMux, s.C.AdminEmails)
	userWithSubMux := wrappers.NewUserWithSubMux(userMux, rSrv)
	userRepoMux := wrappers.NewUserRepoMux(s.C.Name, userWithSubMux, sDb, rSrv, userSrv)
	cliKeyAuthMux := wrappers.NewCliKeyAuthMux(s.C.Name, sDb, rSrv, userSrv, s.mux)
	userWithReadPermMux := wrappers.NewUserWithReadPermissionMux(
		s.C.Name, s.mux, sessionService, sDb, rSrv, userSrv)
	userRepoPipelineMux := wrappers.NewUserRepoPipelineMux(userRepoMux, jobsService)
	serverKeyAuthTrackMux := wrappers.NewServerKeyAuthTrackMux(s.C.TwiggServerKey, s.mux)
	serverKeyAndTokenAuthTrackMux := wrappers.NewServerKeyAndTokenAuthTrackMux(s.C.TwiggServerKey, tokenSigner, s.mux)
	orgOwnerMux := wrappers.NewOrgOwnerMux(s.C.Name, userWithSubMux, sDb, userSrv)
	orgOwnerOrMemberMux := wrappers.NewOrgOwnerOrMemberMux(s.C.Name, userWithSubMux, sDb, userSrv)

	// Register handlers to the muxes
	s.mux.HandleFunc("GET /health",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	s.mux.HandleFunc("GET /which-config-please",
		// Yeah I know this is a silly path, but at least it helps keep some bots away
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(s.C.Name))
		})
	if s.C.RegisterFakeOauthHandler {
		s.mux.HandleFunc("GET /fake-oauth",
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("ok"))
			})
	}
	landing.AddHandlers(rt, s.mux, nil)
	bundles.AddHandler(s.mux)
	webcomponents.AddHandler(s.mux)
	termsandprivacy.AddHandlers(s.mux)
	admindash.AddHandlers(mService, userSrv, queueStorage, s.QueueRunner, adminMux)
	bin.AddHandlers(s.mux)
	login.AddHandlers(s.C.AllowPasswordLogin,
		rt, sDb, userSrv, sessionService, s.mux)
	home.AddHandlers(sDb, rt, rSrv, userSrv, userWithSubMux)
	canCreateRepo := userCanCreateRepoAdaptor{
		repoService: rSrv,
		userService: userSrv,
	}
	newrepo.AddHandlers(canCreateRepo, rSrv, sDb, userWithSubMux)
	reposettings.AddHandlers(userRepoMux, cliKeyAuthMux, userSrv, sDb, rSrv, s.QueueRunner, mirrorSrv, secretsSrv)
	repository.AddHandlers(rSrv, revSrv, userSrv, userWithReadPermMux)
	twigg.AddHandlers(sDb, userSrv, rSrv, ciQueue, s.mux, s.C.Name, tokenSigner, nil)
	commit.AddHandlers(rt, sDb, rSrv, revSrv, userSrv, jobsService,
		ciQueue, ciCdFileParser, trackClient, userRepoMux, userWithReadPermMux,
		s.QueueRunner, canSubCache, s.C.Name)
	payments.AddHandlers(sDb, sessionService, userSrv, stripeClient,
		orgHelper, s.mux, userMux, userWithSubMux)
	plans.AddHandlers(userMux, stripeClient, userSrv)
	usersettings.AddHandlers(userSrv, rSrv, keysService, trackQueue,
		userMux, userWithSubMux)
	usereducation.AddHandlers(sDb, userMux)
	welcome.AddHandlers(authMux)
	needupgrade.AddHandlers(authMux)
	track.AddHandlers(s.C.Name, jobsService, sDb, trackQueue, s.trackObserver,
		secretsSrv, ciQueue, serverKeyAuthTrackMux, serverKeyAndTokenAuthTrackMux)
	jobshandler.AddHandlers(userSrv, jobsService, trackClient, ciService, userRepoMux, userRepoPipelineMux)
	notifications.AddHandlers(rt, sDb, userWithSubMux)
	organization.AddHandlers(userWithSubMux, orgOwnerMux, orgOwnerOrMemberMux, userSrv, sDb, stripeClient, trackQueue, orgHelper, rSrv)
	docusaurus.AddDocsHandler(s.C.Name, s.mux)

	// Start the queues and the server
	s.QueueRunner.Start()
	trackQueue.Start()
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(s.C.Port),
		Handler: s.mux,
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		panic(fmt.Sprintf("failed to start listening: %s", err))
	}
	memLogger.Start()

	go func() {
		log.Printf("TwiggWeb Build Version: %s\n", buildmeta.Version)
		log.Printf("TwiggWeb is serving on http://localhost:%v\n", s.C.Port)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
		log.Printf("TwiggWeb stopped serving on http://localhost:%v\n", s.C.Port)
	}()
	s.IsReady = true

	// --- Listen for OS shutdown signals ---
	// signal.NotifyContext creates a context that is canceled on
	// SIGINT or SIGTERM
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // Clean up signal handling

	// Block until a shutdown signal is received
	<-shutdownCtx.Done()
	log.Println("Starting graceful shutdown...")
	trackQueue.Stop()
	memLogger.Stop()
	s.QueueRunner.Stop()

	// --- Create a context with timeout for graceful shutdown ---
	// This ensures HTTP requests and background work don't hang indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // Release resources when shutdown completes

	// --- Shutdown HTTP server gracefully ---
	// Allows in-flight requests to finish within the timeout
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown with error: %q", err)
	}

	closeSdb()
	closeMetricsService()
	closeQueueStorage()
	log.Println("Server shutdown complete.")
}

// MaintenanceModeMsg is used as a response when teh server runsin maintenance mode
const MaintenanceModeMsg = `Sorry, Twigg is under maintenance :(
Come say hello (or yell at us) on Discord: https://discord.com/invite/udpz3faxwQ`
