package tcp

import (
	"encoding/json"
	"fmt"
	"net"
)

func SendProgressUpdate(address string, msg ProgressSyncMessage, token string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	reg := DeviceRegistration{
		Token:    token,
		DeviceID: msg.DeviceID,
	}
	regData, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintln(conn, string(regData)); err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(conn, string(data))
	return err
}
