package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"internal/provider"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	vklog "github.com/virtual-kubelet/virtual-kubelet/log"
	vklogv2 "github.com/virtual-kubelet/virtual-kubelet/log/klogv2"
)

var (
	buildVersion = "N/A"
	buildTime    = "N/A"
	k8sVersion   = "v1.24.3" // This should follow the version of k8s.io/kubernetes we are importing
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	// Setup logs
	vklog.L = vklogv2.New(map[string]interface{}{"source": "virtual-kubelet"})

	// Default provider configuration.
	// TODO: create other mac specific defaults.
	var opts provider.Opts
	provider.SetOpts(&opts)

	// The Kubernetes version.
	// See https://kubernetes.io/docs/setup/release/version-skew-policy/#kubelet
	opts.Version = k8sVersion

	// Setup the provider commands.
	providerCmd := provider.NewCommand(ctx, filepath.Base(os.Args[0]), &opts)
	if err := providerCmd.Execute(); err != nil && errors.Cause(err) != context.Canceled {
		log.G(ctx).Fatal(err)
	}

	// TODO: Add clean up function.
	log.G(ctx).Info("Exit...")

}
