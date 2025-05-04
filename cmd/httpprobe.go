package cmd

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/bpalermo/maestro/internal/util"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// httpProbeCmd represents the httpprobe command
var (
	url                string
	rootCAFile         string
	expectedStatusCode int
	timeout            time.Duration

	httpProbeCmd = &cobra.Command{
		Use:   "httpprobe",
		Short: "Performs a HTTP health check on the specified endpoint",
		Run:   runHttpProbe,
	}
)

func init() {
	rootCmd.AddCommand(httpProbeCmd)

	httpProbeCmd.Flags().StringVar(&url, "url", "https://localhost:8443/", "Health check URL")
	httpProbeCmd.Flags().StringVar(&rootCAFile, "rootCAFile", "/var/maestro/certs/ca.crt", "TLS certificate authority file path.")
	httpProbeCmd.Flags().IntVar(&expectedStatusCode, "expectedStatusCode", http.StatusOK, "Timeout for the HTTP request")
	httpProbeCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Timeout for the HTTP request")
}

func runHttpProbe(_ *cobra.Command, _ []string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger := klog.FromContext(ctx)

	certPool, err := util.LoadCertPool(rootCAFile)
	if err != nil {
		logger.Error(err, "Could not load certificate authority certificate.")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ClientCAs: certPool,
			},
			MaxIdleConns:        1,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 2 * time.Second,
		},
	}

	performHealthCheck(ctx, logger, client)
}

func performHealthCheck(ctx context.Context, logger klog.Logger, client *http.Client) {
	// Create a new request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.Error(err, "Failed to create request")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err, "Failed to make request")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	defer util.MustClose(resp.Body)

	// Check the status code
	if resp.StatusCode != expectedStatusCode {
		logger.Error(err, "Unexpected status code received", "expected", expectedStatusCode, "got", resp.StatusCode)
		klog.FlushAndExit(klog.ExitFlushTimeout, 2)
	}
}
