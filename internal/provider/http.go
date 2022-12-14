// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	nodeapi "github.com/virtual-kubelet/virtual-kubelet/node/api"
)

// AcceptedCiphers is the list of accepted TLS ciphers, with known weak ciphers elided
// Note this list should be a moving target.
var AcceptedCiphers = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
}

func loadTLSConfig(certPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, errors.Wrap(err, "error loading tls certs")
	}

	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites:             AcceptedCiphers,
	}, nil
}

// setupKubeletServer configures and brings up the kubelet API server.
func setupKubeletServer(ctx context.Context, config *Opts, getPodsFromKubernetes nodeapi.PodListerFunc) (_ func(), retErr error) {
	var closers []io.Closer
	cancel := func() {
		for _, c := range closers {
			c.Close()
		}
	}
	defer func() {
		if retErr != nil {
			cancel()
		}
	}()

	// Ensure valid TLS setup.
	if config.ServerCertPath == "" || config.ServerKeyPath == "" {
		log.
			WithField("cert", config.ServerCertPath).
			WithField("key", config.ServerKeyPath).
			Error("TLS certificates are required to serve the kubelet API")
	} else {
		tlsCfg, err := loadTLSConfig(config.ServerCertPath, config.ServerKeyPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load TLS required for serving the kubelet API")
		}

		// Setup path routing.
		r := mux.NewRouter()

		// This matches the behaviour in the reference kubelet
		r.StrictSlash(true)

		r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		})

		// Start the server.
		s := &http.Server{
			Handler:   r,
			TLSConfig: tlsCfg,
		}
		closers = append(closers, s)
	}

	return cancel, nil
}

func serveHTTP(ctx context.Context, s *http.Server, l net.Listener) {
	if err := s.Serve(l); err != nil {
		select {
		case <-ctx.Done():
		default:
			log.WithError(err).Error("failed to setup the kubelet API server")
		}
	}
	l.Close()
}
