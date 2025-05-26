package utils

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// DetectCompression は、与えられた io.Reader の圧縮形式 (gzip, zstd, xz) を検出し、
// 検出した形式に応じたデコーダーでラップした io.ReadCloser を返します。
// 圧縮形式が検出されない場合は、元の io.Reader を io.NopCloser でラップして返します。
func DetectCompression(r io.Reader) (io.ReadCloser, string, error) {
	// 最初の数バイトを読み込んで圧縮形式を判定するためにバッファリングする
	br := bufio.NewReader(r)
	peek, err := br.Peek(6) // gzip, zstd, xz のマジックバイトは最大6バイト
	if err != nil && err != io.EOF {
		return nil, "", err
	}

	// gzip 形式の判定 (マジックバイト: 1f 8b)
	if len(peek) >= 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gzipReader, err := gzip.NewReader(br)
		if err != nil {
			return nil, "gzip", err
		}
		return gzipReader, "gzip", nil
	}

	// zstd 形式の判定 (マジックバイト: 28 b5 2f fd)
	if len(peek) >= 4 && bytes.Equal(peek[:4], []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		zstdReader, err := zstd.NewReader(br)
		if err != nil {
			return nil, "zstd", err
		}
		return io.NopCloser(zstdReader), "zstd", nil
	}

	// xz 形式の判定 (マジックバイト: fd 37 7a 58 5a 00)
	if len(peek) >= 6 && bytes.Equal(peek[:6], []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		xzReader, err := xz.NewReader(br)
		if err != nil {
			return nil, "xz", err
		}
		return io.NopCloser(xzReader), "xz", nil
	}

	// どの圧縮形式も検出されない場合
	return io.NopCloser(br), "", nil
}
