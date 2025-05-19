package blinkyutils

import (
	"os"

	"github.com/Hayao0819/nahi/futils"
)

func UploadToBlinky(server string, repo string, file string) error {
	client, err := GetClient(server)
	if err != nil {
		return err
	}

	// .sig ファイルが存在すれば開く
	var sigfile *os.File
	sigfilePath := file + ".sig"
	if futils.Exists(sigfilePath) {
		sigfile, err = os.Open(sigfilePath)
		if err != nil {
			return err
		}
		defer func() {
			if sigfile != nil {
				sigfile.Close()
			}
		}()
	}

	// メインのパッケージファイルを開く
	pkgfile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		if pkgfile != nil {
			pkgfile.Close()
		}
	}()

	// アップロード処理
	err = client.UploadPackage(repo, file, pkgfile, sigfile)
	if err != nil {
		return err
	}

	return nil
}
