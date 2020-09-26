package relayer

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/params"
)

type DiscoveryHandler struct {
	host    host.Host
	relayer *Relayer
	log     logger.Logger
	ctx     context.Context
}

func (d *DiscoveryHandler) Listen(network.Network, ma.Multiaddr) {}

func (d *DiscoveryHandler) ListenClose(network.Network, ma.Multiaddr) {}

func (d *DiscoveryHandler) Connected(_ network.Network, conn network.Conn) {
	if conn.Stat().Direction != network.DirOutbound {
		return
	}

	strm, err := d.host.NewStream(d.ctx, conn.RemotePeer(), params.SyncProtocolID)
	if err != nil {
		d.log.Errorf("could not open stream for connection: %s", err)
	}

	d.relayer.HandleStream(strm)
}

func (d *DiscoveryHandler) Disconnected(network.Network, network.Conn) {}

func (d *DiscoveryHandler) OpenedStream(network.Network, network.Stream) {}

func (d *DiscoveryHandler) ClosedStream(network.Network, network.Stream) {}

func NewDiscoveryHandler(ctx context.Context, h host.Host, r *Relayer, log logger.Logger) *DiscoveryHandler {
	return &DiscoveryHandler{ctx: ctx, host: h, relayer: r, log: log}
}
