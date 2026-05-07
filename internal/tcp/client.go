package tcp

import (
	"encoding/json"
	"fmt"
	"net"
)

func SendProgressUpdate(address string, update ProgressUpdate) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	data, err := json.Marshal(update)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(conn, string(data))
	return err
}
