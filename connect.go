package stomp

import (
	"fmt"
	"github.com/dynata/stomp/proto"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

func Connect(c net.Conn, options ...func(Option)) (*Session, error) {
	host, _, splitErr := net.SplitHostPort(c.RemoteAddr().String())

	if nil != splitErr {
		return nil, splitErr
	}
	frame := proto.NewFrame(proto.CmdConnect, nil)

	frame.Header.Set(proto.HdrHost, host)
	frame.Header.Set(proto.HdrAcceptVersion, "1.1,1.2")
	frame.Header.Set(proto.HdrHeartBeat, "0,0")

	for _, option := range options {
		option(Option(frame.Header))
	}
	_, frameWrtErr := frame.WriteTo(c)

	if nil != frameWrtErr {
		return nil, frameWrtErr
	}
	frameReader := proto.NewFrameReader(c)

	respFrame, frameRdErr := frameReader.Read()

	if nil != frameRdErr {
		return nil, frameRdErr
	}
	defer respFrame.Body.Close()

	if respFrame.Command == proto.CmdError {
		contentType, ok := respFrame.Header.Get(proto.HdrContentType)
		var err error

		if ok && contentType == "text/plain" {
			body, bodyRdErr := ioutil.ReadAll(respFrame.Body)

			if nil != bodyRdErr {
				err = fmt.Errorf("unable to read frame body: %v", bodyRdErr)
			} else {
				err = fmt.Errorf("%v", body)
			}
		} else {
			err = fmt.Errorf("frame body content type is unreadable")
		}
		return nil, err
	}

	if respFrame.Command != proto.CmdConnected {
		return nil, fmt.Errorf("unexpected frame command. expected %s, got %s", proto.CmdConnected, respFrame.Command)
	}
	version, _ := respFrame.Header.Get(proto.HdrVersion)
	sessionId, _ := respFrame.Header.Get(proto.HdrSession)
	server, _ := respFrame.Header.Get(proto.HdrServer)

	heartBeat, ok := respFrame.Header.Get(proto.HdrHeartBeat)

	if !ok {
		heartBeat = "0,0"
	}
	tx, rx, hbErr := splitHeartBeat(heartBeat)

	if nil != hbErr {
		return nil, hbErr
	}
	proc := process(c, frameReader)

	session := Session{
		Version:     version,
		ID:          sessionId,
		Server:      server,
		connection:  c,
		txHeartBeat: tx,
		rxHeartBeat: rx,
		processor:   proc,
	}
	return &session, nil
}

func splitHeartBeat(value string) (int, int, error) {
	beats := strings.Split(value, ",")

	if len(beats) < 2 {
		return 0, 0, fmt.Errorf("malformed heart beat header: invalid length")
	}
	tx, txErr := strconv.Atoi(beats[0])

	if nil != txErr {
		return 0, 0, fmt.Errorf("malformed rx heart beat header value: %v", txErr)
	}
	rx, rxErr := strconv.Atoi(beats[1])

	if nil != rxErr {
		return 0, 0, fmt.Errorf("malformed tx heart beat header value: %v", rxErr)
	}
	return tx, rx, nil
}
