package blinkyutils

import (
	"errors"

	blinky_clintlib "github.com/BrenekH/blinky/clientlib"
	blinky_clientutil "github.com/BrenekH/blinky/cmd/blinky/util"
)

func GetClient(server string) (*blinky_clintlib.BlinkyClient, error) {
	serverdb, err := blinky_clientutil.ReadServerDB()
	if err != nil {
		return nil, err
	}

	serverInfo, ok := serverdb.Servers[server]
	if !ok {
		return nil, errors.New("server not found in serverdb")
	}
	return blinky_clintlib.New(server, serverInfo.Username, serverInfo.Password)
}
