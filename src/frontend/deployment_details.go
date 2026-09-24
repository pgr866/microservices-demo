package main

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var deploymentDetailsMap map[string]string
var log *logrus.Logger

func init() {
	initializeLogger()
	loadDeploymentDetails()
}

func initializeLogger() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

// loadDeploymentDetails populates the pod hostname shown in the footer.
// Cluster/zone used to come from the GCP metadata server; there's no
// equivalent lookup for this project's stack, so only the hostname (portable
// across any Kubernetes distribution) remains.
func loadDeploymentDetails() {
	deploymentDetailsMap = make(map[string]string)

	podHostname, err := os.Hostname()
	if err != nil {
		log.Error("Failed to fetch the hostname for the Pod", err)
	}

	deploymentDetailsMap["HOSTNAME"] = podHostname

	log.WithField("hostname", podHostname).Debug("Loaded deployment details")
}
