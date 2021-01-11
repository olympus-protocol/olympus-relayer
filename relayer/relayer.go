package relayer

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/p2p"
	"github.com/olympus-protocol/ogen/pkg/params"
	"io"
	"time"
)

type Relayers struct {
	Name  string
	Addrs string
}

type Relayer struct {
	ID        peer.ID
	log       logger.Logger
	ctx       context.Context
	discovery *discovery.RoutingDiscovery
	dht       *dht.IpfsDHT
	params    *params.ChainParams
	host      host.Host
	syncHandler *SyncHandler
}

func (r *Relayer) FindPeers() {
	for _, rendezvous := range r.params.RendevouzStrings {

		go func(rendezvous string) {
			r.log.Infof("staring listening routine for string: %s", rendezvous)
			for {
				peers, err := r.discovery.FindPeers(r.ctx, rendezvous)
				if err != nil {
					break
				}
			peerLoop:
				for {
					select {
					case pi, ok := <-peers:
						if !ok {
							time.Sleep(time.Second * 10)
							break peerLoop
						}
						r.handleNewPeer(pi)
					case <-r.ctx.Done():
						return
					}
				}
			}
		}(rendezvous)
	}

}

func (r *Relayer) Advertise() {
	for v, rendezvous := range r.params.RendevouzStrings {
		r.log.Infof("starting advertising string: %s on versions higher than %d", rendezvous, v)
		discovery.Advertise(r.ctx, r.discovery, rendezvous)
	}
}

func (r *Relayer) handleNewPeer(pi peer.AddrInfo) {
	if pi.ID == r.ID {
		return
	}
	connectedness := r.host.Network().Connectedness(pi.ID)
	if connectedness != network.Connected {
		r.log.Infof("peer found: %s", pi.String())
		err := r.host.Connect(r.ctx, pi)
		if err != nil {
			r.log.Errorf("unable to connect to peer: %s", pi.String())
		}
	}
}

func (r *Relayer) HandleStream(s network.Stream) {
	r.log.Infof("handling messages from peer %s for protocol %s", s.Conn().RemotePeer(), s.Protocol())
}

func (r *Relayer) processMessages(ctx context.Context, net uint32, stream io.Reader, handler func(p2p.Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			break
		}
		msg, err := p2p.ReadMessage(stream, net)
		if err != nil {
			return err
		}

		if err := handler(msg); err != nil {
			return err
		}
	}
}

func NewRelayer(ctx context.Context, h host.Host, log logger.Logger, discovery *discovery.RoutingDiscovery, dht *dht.IpfsDHT, p *params.ChainParams) *Relayer {

	r := &Relayer{
		host:      h,
		ID:        h.ID(),
		log:       log,
		ctx:       ctx,
		discovery: discovery,
		dht:       dht,
		params:    p,
	}

	syncHandler := NewSyncHandler(ctx, h, r, log, p)
	h.Network().Notify(syncHandler)
	r.syncHandler = syncHandler

	h.SetStreamHandler(params.ProtocolDiscoveryID(p.Name), r.HandleStream)

	return r
}
