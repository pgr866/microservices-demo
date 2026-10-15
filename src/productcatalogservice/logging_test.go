package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    logrus.Level
		wantErr bool
	}{
		{in: "", want: logrus.InfoLevel},
		{in: "debug", want: logrus.DebugLevel},
		{in: "info", want: logrus.InfoLevel},
		{in: "WARN", want: logrus.WarnLevel},
		{in: "Error", want: logrus.ErrorLevel},
		// Accepted by logrus.ParseLevel, but not a level every service shares.
		{in: "trace", wantErr: true},
		{in: "warning", wantErr: true},
		{in: "fatal", wantErr: true},
		{in: "panic", wantErr: true},
		{in: "verbose", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseLogLevel(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLogLevel(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLogLine(t *testing.T) {
	var out bytes.Buffer
	log.Out = &out
	t.Cleanup(func() { log.Out = os.Stdout })

	log.WithField("key", "value").Error("something happened")

	var line map[string]any
	if err := json.Unmarshal(out.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, out.String())
	}
	want := map[string]any{"logger": "productcatalogservice", "severity": "error", "message": "something happened", "key": "value"}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v", k, line[k], v)
		}
	}
	if _, ok := line["timestamp"]; !ok {
		t.Errorf("no timestamp in %s", out.String())
	}
}
