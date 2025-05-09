package blinkyutils

import (
	blinky_clintlib "github.com/BrenekH/blinky/clientlib"
	blinky_clientutil "github.com/BrenekH/blinky/cmd/blinky/util"
)

func GetServerList(serverdb *blinky_clientutil.ServerDB) []string {
	var serverList []string
	for server := range serverdb.Servers {
		serverList = append(serverList, server)
	}
	return serverList
}

func GetClient(server string) (*blinky_clintlib.BlinkyClient, error) {
	serverdb, err := blinky_clientutil.ReadServerDB()
	if err != nil {
		return nil, err
	}

	serverInfo := serverdb.Servers[server]
	return blinky_clintlib.New(server, serverInfo.Username, serverInfo.Password)
}
