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

var OlympusRelayers = []Relayers{
	{
		Name:  "Cronos 1",
		Addrs: "/ip4/134.122.28.156/tcp/25000/p2p/12D3KooWDbvbYMzEvRi5CECVHawadaaZLich93GRhaCRRxQHFVcN",
	},
	{
		Name:  "Cronos 2",
		Addrs: "/ip4/159.65.233.200/tcp/25000/p2p/12D3KooWBdwHS4aLQmfzhddfsT3aV9YA8Cy47VWx8ckoYkC8Vwb4",
	},
	{
		Name:  "Evolbyte",
		Addrs: "/ip4/136.244.90.254/tcp/25000/p2p/12D3KooWNTqMjzeGDuyGDLm3kADGM6FWqN4fV2mem38Y1tEoxEdZ",
	},
	{
		Name: "Stepollo",
		Addrs: "/ip4/45.32.184.252/tcp/25000/p2p/12D3KooWStKhuQayJhKKqnA2HLqo2NrEP9dKhD7Ra7GM5TYUhCNW",
	},
}

type Relayer struct {
	ID          peer.ID
	log         logger.Logger
	ctx         context.Context
	discovery   *discovery.RoutingDiscovery
	dht         *dht.IpfsDHT
	params      *params.ChainParams
	syncHandler *SyncHandler
	host        host.Host
}

func (r *Relayer) FindPeers() {
	for _, rendevouz := range r.params.RendevouzStrings {

		go func(rendevouz string) {
			r.log.Infof("staring listening routine for string: %s", rendevouz)
			for {
				peers, err := r.discovery.FindPeers(r.ctx, rendevouz)
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
		}(rendevouz)
	}

}

func (r *Relayer) Advertise() {
	for v, rendevouz := range r.params.RendevouzStrings {
		r.log.Infof("starting advertising string: %s on versions higher than %d", rendevouz, v)
		discovery.Advertise(r.ctx, r.discovery, rendevouz)
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

	syncHandler := NewSyncHandler(ctx, h, r, log)
	h.Network().Notify(syncHandler)
	r.syncHandler = syncHandler

	h.SetStreamHandler(params.SyncProtocolID, r.HandleStream)

	return r
}
