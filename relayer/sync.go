package relayer

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/params"
)

type SyncHandler struct {
	log     logger.Logger
	host    host.Host
	relayer *Relayer
	ctx     context.Context
}

func (s *SyncHandler) Listen(network.Network, ma.Multiaddr) {}

func (s *SyncHandler) ListenClose(network.Network, ma.Multiaddr) {}

func (s *SyncHandler) Connected(_ network.Network, conn network.Conn) {
	if conn.Stat().Direction != network.DirOutbound {
		return
	}

	strm, err := s.host.NewStream(s.ctx, conn.RemotePeer(), params.SyncProtocolID)
	if err != nil {
		s.log.Errorf("could not open stream for connection: %s", err)
	}

	s.relayer.HandleStream(strm)
}

func (s *SyncHandler) Disconnected(network.Network, network.Conn) {}

func (s *SyncHandler) OpenedStream(network.Network, network.Stream) {}

func (s *SyncHandler) ClosedStream(network.Network, network.Stream) {}

func NewSyncHandler(ctx context.Context, h host.Host, r *Relayer, log logger.Logger) *SyncHandler {
	return &SyncHandler{ctx: ctx, host: h, relayer: r, log: log}
}
