package main

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/params"
)

type discoveryHandler struct {
	host    host.Host
	relayer *relayer
	log     logger.Logger
	ctx     context.Context
}

func (d discoveryHandler) Listen(network.Network, ma.Multiaddr) {}

func (d discoveryHandler) ListenClose(network.Network, ma.Multiaddr) {}

func (d discoveryHandler) Connected(_ network.Network, conn network.Conn) {
	if conn.Stat().Direction != network.DirOutbound {
		return
	}

	strm, err := d.host.NewStream(d.ctx, conn.RemotePeer(), params.SyncProtocolID)
	if err != nil {
		d.log.Errorf("could not open stream for connection: %s", err)
	}

	d.relayer.handleStream(strm)
}

func (d discoveryHandler) Disconnected(network.Network, network.Conn) {}

func (d discoveryHandler) OpenedStream(network.Network, network.Stream) {}

func (d discoveryHandler) ClosedStream(network.Network, network.Stream) {}

func NewDiscoveryHandler(ctx context.Context) *discoveryHandler {
	return &discoveryHandler{ctx: ctx}
}
