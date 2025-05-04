package cmd

import (
	"context"
	"net/http"
	"time"

	"github.com/bpalermo/maestro/internal/util"
	"github.com/spf13/cobra"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"k8s.io/klog/v2"
)

// httpProbeCmd represents the httpprobe command
var (
	url                string
	spireSocketPath    string
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

	httpProbeCmd.Flags().StringVar(&url, "url", "https://localhost/", "Health check URL")
	httpProbeCmd.Flags().StringVar(&spireSocketPath, "spireSocketPath", "unix:///spiffe-workload-api/spire-agent.sock", "Provides an address for the Workload API. The value of the SPIFFE_ENDPOINT_SOCKET environment variable will be used if the option is unused.")
	httpProbeCmd.Flags().IntVar(&expectedStatusCode, "expectedStatusCode", http.StatusOK, "Timeout for the HTTP request")
	httpProbeCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Timeout for the HTTP request")
}

func runHttpProbe(_ *cobra.Command, _ []string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger := klog.FromContext(ctx)

	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(spireSocketPath)))
	if err != nil {
		logger.Error(err, "Failed to create request")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	defer util.MustClose(source)

	tlsConfig := tlsconfig.TLSClientConfig(source, tlsconfig.AuthorizeAny())

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
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
