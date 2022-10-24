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
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

// Defaults for root command options
const (
	DefaultNodeName       = "mac-virtual-kubelet"
	DefaultPodSyncWorkers = 10
	DefaultKubeNamespace  = corev1.NamespaceAll
)

// Opts stores all the options for configuring the root virtual-kubelet command.
// It is used for setting flag values.
//
// You can set the default options by creating a new `Opts` struct and passing
// it into `SetDefaultOpts`
type Opts struct {

	// ServerCertPath is the path to the certificate to secure the kubelet API.
	ServerCertPath string

	// ServerKeyPath is the path to the private key to sign the kubelet API.
	ServerKeyPath string

	// Path to the kubeconfig to use to connect to the Kubernetes API server.
	KubeConfigPath string

	// Namespace to watch for pods and other resources
	KubeNamespace string

	// KubernetesURL is the value to set for the KUBERNETES_SERVICE_* Pod env vars.
	KubernetesURL string

	// Sets the port to listen for requests from the Kubernetes API server
	ListenPort int32

	// Node name to use when creating a node in Kubernetes
	NodeName string

	// Number of workers to use to handle pod notifications
	PodSyncWorkers int

	Version string
}

// SetOpts sets default options for unset values on the passed in options struct.
func SetOpts(default_opts *Opts) error {

	if default_opts.NodeName == "" {
		// Get from environment variable.
		default_opts.NodeName = os.Getenv("HOSTNAME")
		if default_opts.NodeName == "" {
			default_opts.NodeName = DefaultNodeName
		}
	}

	if default_opts.PodSyncWorkers <= 0 {
		default_opts.PodSyncWorkers = DefaultPodSyncWorkers
	}

	if default_opts.ListenPort == 0 {
		default_opts.ListenPort = 10248
	}

	if default_opts.KubeNamespace == "" {
		default_opts.KubeNamespace = DefaultKubeNamespace
	}

	if default_opts.KubeConfigPath == "" {
		// Get from environment variable.
		default_opts.KubeConfigPath = os.Getenv("KUBECONFIG")
		if default_opts.KubeConfigPath == "" {
			default_opts.KubeConfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
	}

	return nil
}
