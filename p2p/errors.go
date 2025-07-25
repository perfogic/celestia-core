package p2p

import (
	"fmt"
	"net"
	"strings"
)

// ErrFilterTimeout indicates that a filter operation timed out.
type ErrFilterTimeout struct{}

func (e ErrFilterTimeout) Error() string {
	return "filter timed out"
}

// ErrRejected indicates that a Peer was rejected carrying additional
// information as to the reason.
type ErrRejected struct {
	addr               NetAddress
	conn               net.Conn
	err                error
	id                 ID
	isAuthFailure      bool
	isDuplicate        bool
	isFiltered         bool
	isIncompatible     bool
	isNodeInfoInvalid  bool
	isSelf             bool
	localNodeID        string
	remoteNodeID       string
	localAddr          string
	remoteAddr         string
	handshakeStage     string
	traceID            string
	chainID            string
	peerChainID        string
	malformedHandshake bool
}

// Addr returns the NetAddress for the rejected Peer.
func (e ErrRejected) Addr() NetAddress {
	return e.addr
}

func (e ErrRejected) Error() string {
	var base string
	switch {
	case e.isAuthFailure:
		base = fmt.Sprintf("auth failure: %s", e.err)
	case e.isDuplicate:
		if e.conn != nil {
			base = fmt.Sprintf("duplicate CONN<%s>", e.conn.RemoteAddr().String())
		} else if e.id != "" {
			base = fmt.Sprintf("duplicate ID<%v>", e.id)
		}
	case e.isFiltered:
		if e.conn != nil {
			base = fmt.Sprintf("filtered CONN<%s>: %s", e.conn.RemoteAddr().String(), e.err)
		} else if e.id != "" {
			base = fmt.Sprintf("filtered ID<%v>: %s", e.id, e.err)
		}
	case e.isIncompatible:
		base = fmt.Sprintf("incompatible: %s", e.err)
	case e.isNodeInfoInvalid:
		base = fmt.Sprintf("invalid NodeInfo: %s", e.err)
	case e.isSelf:
		base = fmt.Sprintf("self ID<%v>", e.id)
	default:
		base = fmt.Sprintf("%s", e.err)
	}

	fields := []string{base}
	if e.localNodeID != "" {
		fields = append(fields, "localNodeID="+e.localNodeID)
	}
	if e.remoteNodeID != "" {
		fields = append(fields, "remoteNodeID="+e.remoteNodeID)
	}
	if e.localAddr != "" {
		fields = append(fields, "localAddr="+e.localAddr)
	}
	if e.remoteAddr != "" {
		fields = append(fields, "remoteAddr="+e.remoteAddr)
	}
	if e.handshakeStage != "" {
		fields = append(fields, "handshakeStage="+e.handshakeStage)
	}
	if e.traceID != "" {
		fields = append(fields, "traceID="+e.traceID)
	}
	if e.chainID != "" {
		fields = append(fields, "chainID="+e.chainID)
	}
	if e.peerChainID != "" {
		fields = append(fields, "peerChainID="+e.peerChainID)
	}
	if e.malformedHandshake {
		fields = append(fields, "malformed_handshake=true")
	}
	return strings.Join(fields, " | ")
}

// IsAuthFailure when Peer authentication was unsuccessful.
func (e ErrRejected) IsAuthFailure() bool { return e.isAuthFailure }

// IsDuplicate when Peer ID or IP are present already.
func (e ErrRejected) IsDuplicate() bool { return e.isDuplicate }

// IsFiltered when Peer ID or IP was filtered.
func (e ErrRejected) IsFiltered() bool { return e.isFiltered }

// IsIncompatible when Peer NodeInfo is not compatible with our own.
func (e ErrRejected) IsIncompatible() bool { return e.isIncompatible }

// IsNodeInfoInvalid when the sent NodeInfo is not valid.
func (e ErrRejected) IsNodeInfoInvalid() bool { return e.isNodeInfoInvalid }

// IsSelf when Peer is our own node.
func (e ErrRejected) IsSelf() bool { return e.isSelf }

// ErrSwitchDuplicatePeerID to be raised when a peer is connecting with a known
// ID.
type ErrSwitchDuplicatePeerID struct {
	ID ID
}

func (e ErrSwitchDuplicatePeerID) Error() string {
	return fmt.Sprintf("duplicate peer ID %v", e.ID)
}

// ErrSwitchDuplicatePeerIP to be raised whena a peer is connecting with a known
// IP.
type ErrSwitchDuplicatePeerIP struct {
	IP net.IP
}

func (e ErrSwitchDuplicatePeerIP) Error() string {
	return fmt.Sprintf("duplicate peer IP %v", e.IP.String())
}

// ErrSwitchConnectToSelf to be raised when trying to connect to itself.
type ErrSwitchConnectToSelf struct {
	Addr *NetAddress
}

func (e ErrSwitchConnectToSelf) Error() string {
	return fmt.Sprintf("connect to self: %v", e.Addr)
}

type ErrSwitchAuthenticationFailure struct {
	Dialed *NetAddress
	Got    ID
}

func (e ErrSwitchAuthenticationFailure) Error() string {
	return fmt.Sprintf(
		"failed to authenticate peer. Dialed %v, but got peer with ID %s",
		e.Dialed,
		e.Got,
	)
}

// ErrTransportClosed is raised when the Transport has been closed.
type ErrTransportClosed struct{}

func (e ErrTransportClosed) Error() string {
	return "transport has been closed"
}

// ErrPeerRemoval is raised when attempting to remove a peer results in an error.
type ErrPeerRemoval struct{}

func (e ErrPeerRemoval) Error() string {
	return "peer removal failed"
}

//-------------------------------------------------------------------

type ErrNetAddressNoID struct {
	Addr string
}

func (e ErrNetAddressNoID) Error() string {
	return fmt.Sprintf("address (%s) does not contain ID", e.Addr)
}

type ErrNetAddressInvalid struct {
	Addr string
	Err  error
}

func (e ErrNetAddressInvalid) Error() string {
	return fmt.Sprintf("invalid address (%s): %v", e.Addr, e.Err)
}

type ErrNetAddressLookup struct {
	Addr string
	Err  error
}

func (e ErrNetAddressLookup) Error() string {
	return fmt.Sprintf("error looking up host (%s): %v", e.Addr, e.Err)
}

// ErrCurrentlyDialingOrExistingAddress indicates that we're currently
// dialing this address or it belongs to an existing peer.
type ErrCurrentlyDialingOrExistingAddress struct {
	Addr string
}

func (e ErrCurrentlyDialingOrExistingAddress) Error() string {
	return fmt.Sprintf("connection with %s has been established or dialed", e.Addr)
}
