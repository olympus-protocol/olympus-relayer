package main

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/params"
)

type syncHandler struct {
	log     logger.Logger
	host    host.Host
	relayer *relayer
	ctx     context.Context
}

func (s syncHandler) Listen(network.Network, ma.Multiaddr) {}

func (s syncHandler) ListenClose(network.Network, ma.Multiaddr) {}

func (s syncHandler) Connected(_ network.Network, conn network.Conn) {
	if conn.Stat().Direction != network.DirOutbound {
		return
	}

	strm, err := s.host.NewStream(s.ctx, conn.RemotePeer(), params.SyncProtocolID)
	if err != nil {
		s.log.Errorf("could not open stream for connection: %s", err)
	}

	s.relayer.handleStream(strm)
}

func (s syncHandler) Disconnected(network.Network, network.Conn) {}

func (s syncHandler) OpenedStream(network.Network, network.Stream) {}

func (s syncHandler) ClosedStream(network.Network, network.Stream) {}

func NewSyncHandler(ctx context.Context) *syncHandler {
	return &syncHandler{ctx: ctx}
}
