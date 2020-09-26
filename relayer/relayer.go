package relayer

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/p2p"
	"github.com/olympus-protocol/ogen/pkg/params"
	"io"
	"sync"
	"time"
)

var topicsSubs = []string{
	p2p.MsgBlockCmd,
	p2p.MsgTxCmd,
	p2p.MsgDepositCmd,
	p2p.MsgDepositsCmd,
	p2p.MsgVoteCmd,
	p2p.MsgValidatorStartCmd,
	p2p.MsgExitCmd,
	p2p.MsgExitsCmd,
	p2p.MsgGovernanceCmd,
	p2p.MsgTxMultiCmd,
	p2p.MsgGetBlocksCmd,
}

type Topics struct {
	Pubsub     *pubsub.PubSub
	Topics     map[string]*pubsub.Topic
	topicsLock sync.RWMutex
}

type Relayer struct {
	ID          peer.ID
	log         logger.Logger
	ctx         context.Context
	discovery   *discovery.RoutingDiscovery
	dht         *dht.IpfsDHT
	params      *params.ChainParams
	topics      *Topics
	syncHandler *SyncHandler
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
						fmt.Println(pi, ok)
						if !ok {
							time.Sleep(time.Second * 10)
							break peerLoop
						}
						r.log.Tracef("Advertised peer found %s handling...", pi.ID)
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
		r.log.Infof("starting advertizing string: %s on versions higher than %d", rendevouz, v)
		discovery.Advertise(r.ctx, r.discovery, rendevouz)
	}
}

func (r *Relayer) handleNewPeer(pi peer.AddrInfo) {
	if pi.ID == r.ID {
		return
	}
	r.log.Debugf("peer found: %s", pi.String())
}

func (r *Relayer) Subscribe() {
	for _, topic := range topicsSubs {
		r.log.Debugf("subscribing and relaying on topic %s", topic)
		t, err := r.topics.Pubsub.Join(topic)
		if err != nil {
			r.log.Fatal(err)
		}
		_, err = t.Relay()
		if err != nil {
			r.log.Fatal(err)
		}
	}

}

func (r *Relayer) HandleStream(s network.Stream) {
	r.log.Infof("handling messages from peer %s for protocol %s", s.Conn().RemotePeer(), s.Protocol())
	go r.receiveMessages(s.Conn().RemotePeer(), s)
}

func (r *Relayer) receiveMessages(id peer.ID, reader io.Reader) {
	_ = r.processMessages(r.ctx, r.params.NetMagic, reader, func(message p2p.Message) error {
		cmd := message.Command()

		r.log.Tracef("processing message %s from peer %s", cmd, id)

		r.topics.topicsLock.Lock()
		defer r.topics.topicsLock.Unlock()
		topic, ok := r.topics.Topics[cmd]
		if !ok {
			return nil
		}
		msg, err := message.Marshal()
		if err != nil {
			return nil
		}
		return topic.Publish(r.ctx, msg)
	})
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
	g, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatal(err)
	}

	t := &Topics{
		Pubsub: g,
		Topics: make(map[string]*pubsub.Topic),
	}

	r := &Relayer{
		ID:        h.ID(),
		log:       log,
		ctx:       ctx,
		discovery: discovery,
		dht:       dht,
		params:    p,
		topics:    t,
	}

	syncHandler := NewSyncHandler(ctx, h, r, log)
	h.Network().Notify(syncHandler)
	r.syncHandler = syncHandler

	h.SetStreamHandler(params.DiscoveryProtocolID, r.HandleStream)
	h.SetStreamHandler(params.SyncProtocolID, r.HandleStream)

	return r
}
