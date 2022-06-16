package main

import (
	"os"
	"strings"

	"github.com/govirtuo/cf2ovh/cloudflare"
	"github.com/govirtuo/cf2ovh/ovh"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) != 2 {
		logrus.Fatal("no zone name provided")
	}
	zoneName := os.Args[1]

	err := godotenv.Load()
	if err != nil {
		logrus.Error("cannot load .env file, will look in env anyway")
	}

	ccf := cloudflare.Credentials{
		AuthEmail: os.Getenv("CLOUDFLARE_AUTH_EMAIL"),
		AuthKey:   os.Getenv("CLOUDFLARE_AUTH_KEY"),
	}

	covh := ovh.Credentials{
		ApplicationKey:    os.Getenv("OVH_APPLICATION_KEY"),
		ApplicationSecret: os.Getenv("OVH_APPLICATION_SECRET"),
		ConsumerKey:       os.Getenv("OVH_CONSUMER_KEY"),
	}

	logrus.Infof("getting %s zone ID on Cloudflare API", zoneName)
	id, err := cloudflare.GetZoneID(zoneName, ccf)
	if err != nil {
		logrus.Fatalf("cannot get zone ID: %s\n", err)
	}

	logrus.Infof("getting new TXT records for zone ID %s on Cloudflare API", id)
	vals, err := cloudflare.GetTXTValues(id, ccf)
	if err != nil {
		logrus.Fatalf("cannot get TXT validation records: %s", err)
	}

	subdomain := "_acme-challenge"
	if zoneName != os.Getenv("BASE_DOMAIN") {
		subdomain = strings.TrimSuffix("_acme-challenge."+zoneName, "."+os.Getenv("BASE_DOMAIN"))
	}

	logrus.Infof("getting IDs for subdomain %s on OVH API", subdomain)
	ids, err := ovh.GetDomainIDs(subdomain, covh)
	if err != nil {
		logrus.Fatalf("cannot get IDs for domain %s: %s", subdomain, err)
	}

	for i, v := range vals {
		logrus.Infof("updating %s (ID: %s) with value: %s", subdomain, ids[i], v.TxtValue)
		if err := ovh.UpdateTXTRecord(ids[i], v.TxtValue, subdomain, covh); err != nil {
			logrus.Fatalf("TXT record update failed: %s", err)
		}
	}

	logrus.Infof("retriggering SSL verification on Cloudflare API")
	if err := cloudflare.RetriggerSSLVerification(id, ccf); err != nil {
		logrus.Fatalf("cannot retrigger SSL verification on zone ID %s: %s", id, err)
	}
}
