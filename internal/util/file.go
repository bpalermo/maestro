package util

import (
	"io"
	"os"

	"github.com/rs/zerolog/log"
)

func WriteData(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	if err != nil {
		log.Debug().Msg("write data failed")
		panic(err)
		return err
	}
	return nil
}

func MustCloseFile(f *os.File) {
	err := f.Close()
	if err != nil {
		panic(err)
	}
}
