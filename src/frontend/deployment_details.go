package main

import "os"

var deploymentDetailsMap map[string]string

// loadDeploymentDetails populates the pod hostname shown in the footer.
// Cluster/zone used to come from the GCP metadata server; there's no
// equivalent lookup for this project's stack, so only the hostname (portable
// across any Kubernetes distribution) remains.
func loadDeploymentDetails() {
	deploymentDetailsMap = make(map[string]string)

	podHostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Error("failed to fetch the hostname for the Pod")
	}

	deploymentDetailsMap["HOSTNAME"] = podHostname

	log.WithField("hostname", podHostname).Debug("Loaded deployment details")
}
