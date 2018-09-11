package types

import (
	"net"
	"os"
)

var SocketName = "machinehead.sock"

// SocketExists checks if the daemon is already running
func SocketExists() (exists bool) {
	_, err := os.Stat(SocketName)
	if err != nil {
		return false
	}
	return true
}

// GetSocket returns the IPC socket for communicating with an existing daemon
func GetSocket() (conn net.Conn, err error) {
	conn, err = net.Dial("unix", SocketName)
	if err != nil {
		return
	}

	return
}
