package main

import (
	"flag"
	"fmt"
	"monorepo/twigg-web/server"
	"monorepo/twigg-web/srvconfig"
	"monorepo/twiggbuildflags"
)

// If IsMaintenanceMode build flag is set, the server will run
// in maintenance mode: its a very simple http server that just serves a msg
var RunInMaintenanceMode = twiggbuildflags.IsMaintenanceMode != "false"

var (
	port                 = flag.Int("p", 9001, "port used to run the server")
	storageFolderAbsPath = flag.String("s", "", "absolute path to storage folder")
	config               = flag.String("config", "", "local/mock/prod")
	trackServerUrl       = flag.String("track-url", "", "full url to track server")
)

func main() {
	flag.Parse()

	// Keys used by the configs that talk to a twigg-track running locally.
	// They must match what twigg-track's main uses for its local configs.
	const (
		localTwiggServerKey = "twigg-server-key"
		localTrackKey       = "track-key"
	)

	var serverConfig srvconfig.SrvConfig
	switch *config {
	case "mock":
		serverConfig = srvconfig.MockConfig(*port, *storageFolderAbsPath, localTwiggServerKey, *trackServerUrl, localTrackKey)
	case "prod":
		serverConfig = srvconfig.ProdConfig(*port, *storageFolderAbsPath, *trackServerUrl)
	case "local":
		serverConfig = srvconfig.LocalConfig(*port, *storageFolderAbsPath, localTwiggServerKey, *trackServerUrl, localTrackKey)
	case "lab":
		serverConfig = srvconfig.HomelabConfig(*port, *storageFolderAbsPath, *trackServerUrl)
	case "":
		fmt.Printf("-config flag not provided\n")
		return
	default:
		fmt.Printf("invalid config provided: %q\n", *config)
		return
	}

	srv := server.NewSrv(serverConfig)
	srv.Run(RunInMaintenanceMode)
}
