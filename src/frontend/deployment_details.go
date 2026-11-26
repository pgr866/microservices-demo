package main

import "os"

// podHostname, shown in the footer, tells which replica served a page.
var podHostname string

func loadDeploymentDetails() {
	var err error
	podHostname, err = os.Hostname()
	if err != nil {
		log.WithError(err).Error("failed to fetch the hostname for the Pod")
	}
	log.WithField("hostname", podHostname).Debug("Loaded deployment details")
}
