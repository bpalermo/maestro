package util

import (
	"io"

	"k8s.io/klog/v2"
)

func MustClose(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		klog.Fatal(err, "failed to close closer")
	}
}
