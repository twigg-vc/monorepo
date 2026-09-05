// This file contains helper functions for initializing the server
package server

import (
	"fmt"
	"monorepo/data/fileblobstore"
	"monorepo/twigg-web/services/digitalocean"
	"monorepo/twigg-web/services/oauthclient"
	"monorepo/twigg-web/services/session"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/srvconfig"
	"monorepo/twigg-web/webdb"
	"time"
)

// Parse and verify storageFolderAbsPath to instantiate the db.
// Panics on error and logs the error.
func getDb(c srvconfig.SrvConfig) (db webdb.WebDb, closeDb func()) {
	var blobStorage webdb.BlobStorage
	if c.UseDigitalOceanSpaces {
		blobStorage = digitalocean.NewSpacesClient(
			c.DigitalOceanSpacesBucketUrl,
			c.DigitalOceanSpacesFolderName,
			c.DigitalOceanSpacesAccessKeyId,
			c.DigitalOceanSpacesAccessKeySecret,
		)
	} else {
		var err error
		blobStorage, err = fileblobstore.NewFileBlobStorage(c.StorageFolderAbsPath)
		if err != nil {
			panic(fmt.Sprintf("unable to create blobstore: %s", err))
		}
	}
	db, closeDb, err := webdb.New(
		c.StorageFolderAbsPath, "sqlarge.db", c.StorageBlockSize, blobStorage,
		c.BlobStorageCacheCapacity,
		/*enforceQuota*/ !c.DisableQuotaEnforcement)
	if err != nil {
		panic(fmt.Sprintf("unable to initialize db under %s: %s", c.StorageFolderAbsPath, err))
	}
	return
}

func (s Srv) getSessionService(
	db webdb.WebDb, userSrv userservice.Service,
	googleOAuthClient oauthclient.Google,
	microsoftOAuthClient oauthclient.Microsoft) session.Service {
	if s.C.MockAuthUser && s.C.InsecureAuthCookies {
		panic("cant use MockAuthUser together with InsecureAuthCookies")
	}
	if s.C.MockAuthUser && s.C.DisableStrongCsrfProtection {
		panic("cant use MockAuthUser together with DisableStrongCsrfProtection")
	}
	if s.C.MockAuthUser {
		const mockedAuthUsername = "aang"
		r, cl, err := db.BeginRead()
		defer cl()
		if err != nil {
			panic(fmt.Sprintf(
				"could not get tx to verify mocked user exists: %s", err))
		}
		u, _, err := userSrv.GetByUsername(r, mockedAuthUsername)
		if err != nil {
			panic(fmt.Sprintf(
				"could not verify the mocked user exists: %s", err))
		}
		return session.NewFake(u.Id, u.Username)
	}
	var opts []session.ServiceOption
	if s.C.DisableStrongCsrfProtection {
		opts = []session.ServiceOption{}
	} else {
		opts = []session.ServiceOption{session.WithStrongCsrfProtection()}
	}
	if s.C.InsecureAuthCookies {
		return session.NewInsecureCookiesService(time.Duration(20*time.Hour),
			googleOAuthClient, microsoftOAuthClient, opts...)
	}
	return session.NewService(time.Duration(20*time.Hour),
		googleOAuthClient, microsoftOAuthClient, opts...)
}
