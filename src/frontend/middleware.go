// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"net/http"
	"os"
	"time"
	"uuid"

	"github.com/sirupsen/logrus"
)

type ctxKeyLog struct{}
type ctxKeyRequestID struct{}

type logHandler struct {
	log  *logrus.Logger
	next http.Handler
}

type responseRecorder struct {
	b      int
	status int
	w      http.ResponseWriter
}

func (r *responseRecorder) Header() http.Header { return r.w.Header() }

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.w.Write(p)
	r.b += n
	return n, err
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.w.WriteHeader(statusCode)
}

func (lh *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := uuid.New()
	ctx = context.WithValue(ctx, ctxKeyRequestID{}, requestID.String())

	start := time.Now()
	rr := &responseRecorder{w: w}
	log := lh.log.WithFields(logrus.Fields{
		"http.req.path":   r.URL.Path,
		"http.req.method": r.Method,
		"http.req.id":     requestID.String(),
	})
	// No session ID: it is the only key to the user's cart, so it stays out of
	// the logs. http.req.id already ties together the lines of a request.
	log.Debug("request started")
	defer func() {
		log.WithFields(logrus.Fields{
			"http.resp.took_ms": int64(time.Since(start) / time.Millisecond),
			"http.resp.status":  rr.status,
			"http.resp.bytes":   rr.b}).Debugf("request complete")
	}()

	ctx = context.WithValue(ctx, ctxKeyLog{}, log)
	r = r.WithContext(ctx)
	lh.next.ServeHTTP(rr, r)
}

// maxBodyBytes caps the body of a request: the largest form, the checkout one,
// is well under 1 KiB.
const maxBodyBytes = 64 << 10

// contentSecurityPolicy lets the pages load scripts and images only from this
// site, styles also from the Bootstrap CDN and Google Fonts, and fonts from
// Google Fonts, so a script injected into a page can't run or send data out.
// "data:" images are the icons inlined in Bootstrap's CSS.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' https://cdn.jsdelivr.net https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; " +
	"img-src 'self' data:; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// securityHeaders sets the headers that tell the browser to apply the policy
// above, not to guess content types, not to show the shop inside another
// site's frame (clickjacking), and not to send the page address to other sites.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// "same-origin" still sends it within the shop, which setCurrency
		// needs to go back to the page the user was on.
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// limitBody stops reading a request body past maxBodyBytes, so a huge upload
// can't make the server buffer it whole while parsing a form.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// withTimeout gives every request a deadline. The handlers pass the request
// context to their gRPC calls, so the deadline reaches every service down the
// chain (checkoutservice and recommendationservice pass it on), and a request
// stuck on a dependency fails with 504 instead of waiting indefinitely.
func withTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ensureSessionID(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sessionID string
		// r.Cookie only fails when there is no such cookie.
		if c, err := r.Cookie(cookieSessionID); err == nil {
			sessionID = c.Value
		} else {
			if os.Getenv("ENABLE_SINGLE_SHARED_SESSION") == "true" {
				// Hard coded user id, shared across sessions
				sessionID = "12345678-1234-1234-1234-123456789123"
			} else {
				sessionID = uuid.New().String()
			}
			// HttpOnly: no script needs the session ID, so an injected one
			// can't read it either. SameSite=Lax: other sites can't send it
			// along with their requests (CSRF), except for plain links.
			http.SetCookie(w, &http.Cookie{
				Name:     cookieSessionID,
				Value:    sessionID,
				MaxAge:   cookieMaxAge,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}
		ctx := context.WithValue(r.Context(), ctxKeySessionID{}, sessionID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
}
