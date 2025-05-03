package cmd

import (
	"context"
	"net/http"
	"time"

	"github.com/bpalermo/maestro/internal/util"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// httpProbeCmd represents the httpprobe command
var (
	url                string
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

	httpProbeCmd.Flags().StringVar(&url, "url", "http://localhost/", "Health check URL")
	httpProbeCmd.Flags().IntVar(&expectedStatusCode, "expectedStatusCode", http.StatusOK, "Timeout for the HTTP request")
	httpProbeCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Timeout for the HTTP request")
}

func runHttpProbe(cmd *cobra.Command, args []string) {
	klog.InitFlags(nil)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger := klog.FromContext(ctx)

	client := &http.Client{
		Timeout: timeout,
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
