package blinkyutils

import (
	blinky_clientutil "github.com/BrenekH/blinky/cmd/blinky/util"
)

func GetServerList(serverdb *blinky_clientutil.ServerDB) []string {
	var serverList []string
	for server := range serverdb.Servers {
		serverList = append(serverList, server)
	}
	return serverList
}

