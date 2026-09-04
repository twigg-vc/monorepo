package main

import (
	"flag"
	"fmt"
	"log"
	"monorepo/twigg-track/trackserver"
	"monorepo/twigg-web/routes"
	twiggwebclient "monorepo/twigg-web/twigg-web-client"
)

var (
	port                 = flag.Int("p", 2001, "port used to run the server")
	storageFolderAbsPath = flag.String("s", "", "absolute path to storage folder")
	twiggServerUrl       = flag.String("twigg-server-url", "URL NOT PROVIDED", "url to post webhooks")
	config               = flag.String("config", "", "mock/test/local/lab/prod")
)

func main() {
	log.Printf("Twigg track is starting...")
	flag.Parse()

	const twiggServerWebhookPath = routes.TrackWebhooksPath

	// Keys used by the configs that talk to a twigg-web running locally.
	// They must match what twigg-web's main uses for its local configs.
	const (
		localTwiggServerKey = "twigg-server-key"
		localTrackKey       = "track-key"
	)

	var serverConfig trackserver.SrvConfig
	switch *config {
	case "mock":
		serverConfig = trackserver.MockConfig(*port, *storageFolderAbsPath, *twiggServerUrl, twiggServerWebhookPath, localTwiggServerKey, localTrackKey)
	case "test":
		serverConfig = trackserver.TestConfig(*port, *storageFolderAbsPath, *twiggServerUrl, twiggServerWebhookPath, localTwiggServerKey, localTrackKey)
	case "prod":
		serverConfig = trackserver.ProdConfig(*port, *storageFolderAbsPath, *twiggServerUrl, twiggServerWebhookPath)
	case "local":
		serverConfig = trackserver.LocalConfig(*port, *storageFolderAbsPath, *twiggServerUrl, twiggServerWebhookPath, localTwiggServerKey, localTrackKey)
	case "lab":
		serverConfig = trackserver.HomelabConfig(*port, *storageFolderAbsPath, *twiggServerUrl, twiggServerWebhookPath)
	case "":
		fmt.Printf("-config flag not provided\n")
		return
	default:
		fmt.Printf("invalid config provided: %q\n", *config)
		return
	}

	twiggWebClient := twiggwebclient.NewClient(serverConfig.TwiggServerUrl, serverConfig.TwiggServerKey)

	srv := trackserver.NewSrv(serverConfig, twiggWebClient)
	srv.Run()
}
