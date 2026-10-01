package main

import "os"

var deploymentDetailsMap map[string]string

// loadDeploymentDetails sets the pod hostname shown in the footer.
func loadDeploymentDetails() {
	deploymentDetailsMap = make(map[string]string)

	podHostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Error("failed to fetch the hostname for the Pod")
	}

	deploymentDetailsMap["HOSTNAME"] = podHostname

	log.WithField("hostname", podHostname).Debug("Loaded deployment details")
}
