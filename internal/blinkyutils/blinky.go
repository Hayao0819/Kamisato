package blinkyutils

import (
	"os"
	"strings"

	blinky_clientlib "github.com/BrenekH/blinky/clientlib"
)

type Client struct {
	upstream *blinky_clientlib.BlinkyClient
}

type ServerInfo struct {
	URL      string
	Username string
	Password string
}

func (s *ServerInfo) Client() (*Client, error) {
	upstream, err := blinky_clientlib.New(strings.TrimRight(s.URL, "/")+"/blinky", s.Username, s.Password)
	if err != nil {
		return nil, err
	}
	return &Client{upstream: upstream}, nil
}

func Upload(client *Client, repo, pkgPath, sigPath string) error {
	pkgFile, err := os.Open(pkgPath)
	if err != nil {
		return err
	}
	defer func() { _ = pkgFile.Close() }()

	var sigFile *os.File
	if sigPath != "" {
		sigFile, err = os.Open(sigPath)
		if err != nil {
			return err
		}
		defer func() { _ = sigFile.Close() }()
	}

	if sigFile == nil {
		return client.upstream.UploadPackage(repo, pkgPath, pkgFile, nil)
	}
	return client.upstream.UploadPackage(repo, pkgPath, pkgFile, sigFile)
}
