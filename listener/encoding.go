package listener

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"log/slog"
)

func newEncodedReader(enc string, r io.ReadCloser) (io.ReadCloser, error) {
	switch enc {
	case "":
		return r, nil
	case "gzip":
		return gzip.NewReader(r)
	case "deflate":
		return zlib.NewReader(r)
	case "compress", "br":
		return nil, fmt.Errorf("unsupported encoding %q", enc)
	default:
		slog.Warn("unknown encoding", "enc", enc)
		return r, nil
	}
}

func readAllEncoded(enc string, r io.ReadCloser) ([]byte, error) {
	d, err := newEncodedReader(enc, r)
	if err == io.EOF {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	bs, err := io.ReadAll(d)
	if err != nil {
		return nil, err
	}

	if err := d.Close(); err != nil {
		slog.Warn("could not close reader", "err", err)
	}

	return bs, nil
}
