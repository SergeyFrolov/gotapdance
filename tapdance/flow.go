package tapdance

import (
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	MSG_DATA = iota // iota auto increments
	MSG_INIT
	MSG_RECONNECT
	MSG_CLOSE
)

// Connection-oriented state
type TapDanceFlow struct {
	// tunnel index and start time
	id      uint64
	startMs time.Time //TODO: unused

	// reference to global proxy
	proxy *TapdanceProxy

	servConn *tapdanceConn
	userConn net.Conn
}

// constructor
func NewTapDanceFlow(proxy *TapdanceProxy, id uint64) *TapDanceFlow {
	state := new(TapDanceFlow)

	state.proxy = proxy
	state.id = id

	state.startMs = time.Now()

	Logger.Debugf("Created new TDState ", state)
	return state
}

func (TDstate *TapDanceFlow) Redirect() (err error) {
	TDstate.servConn, err = DialTapDance(TDstate.id, nil)
	if err != nil {
		TDstate.userConn.Close()
		return
	}
	errChan := make(chan error)
	defer func() {
		TDstate.userConn.Close()
		TDstate.servConn.Close()
		_ = <-errChan // wait for second goroutine to close
	}()

	forwardFromServerToClient := func() {
		n, _err := io.Copy(TDstate.userConn, TDstate.servConn)
		Logger.Debugf("[Flow " + strconv.FormatUint(uint64(TDstate.id), 10) +
			"] forwardFromServerToClient returns, bytes sent: " +
			strconv.FormatUint(uint64(n), 10))
		if _err == nil {
			_err = errors.New("!!!Server returned without error")
		}
		errChan <- _err
		return
	}

	forwardFromClientToServer := func() {
		n, _err := io.Copy(TDstate.servConn, TDstate.userConn)
		Logger.Debugf("[Flow " + strconv.FormatUint(uint64(TDstate.id), 10) +
			"] forwardFromClientToServer returns, bytes sent: " +
			strconv.FormatUint(uint64(n), 10))
		if _err == nil {
			_err = errors.New("StoppedByUser")
		}
		errChan <- _err
		return
	}

	go forwardFromServerToClient()
	go forwardFromClientToServer()

	if err = <-errChan; err != nil {
		if err.Error() == "MSG_CLOSE" || err.Error() == "StoppedByUser" {
			Logger.Debugf("[Flow " + strconv.FormatUint(uint64(TDstate.id), 10) +
				"] Redirect function returns gracefully: " + err.Error())
			TDstate.proxy.closedGracefully.inc()
			err = nil
		} else {
			str_err := err.Error()

			// statistics
			if strings.Contains(str_err, "TapDance station didn't pick up the request") {
				TDstate.proxy.notPickedUp.inc()
			} else if strings.Contains(str_err, ": i/o timeout") {
				TDstate.proxy.timedOut.inc()
			} else {
				TDstate.proxy.unexpectedError.inc()
			}
		}
	}
	return
}
