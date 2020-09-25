package main

import (
	"context"
	"crypto/rand"
	dsbadger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/p2p"
	"github.com/olympus-protocol/ogen/pkg/params"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"os"
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

var (
	debug   bool
	port    string
	net     string
	connect string
)

type topics struct {
	pubsub     *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	topicsLock sync.RWMutex
}

type relayer struct {
	ID               peer.ID
	log              logger.Logger
	ctx              context.Context
	discovery        *discovery.RoutingDiscovery
	dht              *dht.IpfsDHT
	params           *params.ChainParams
	topics           *topics
	syncHandler      *syncHandler
	discoveryHandler *discoveryHandler
}

func (r *relayer) findPeers() {
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

func (r *relayer) advertise() {
	for v, rendevouz := range r.params.RendevouzStrings {
		r.log.Infof("starting advertizing string: %s on versions higher than %d", rendevouz, v)
		discovery.Advertise(r.ctx, r.discovery, rendevouz)
	}
}

func (r *relayer) handleNewPeer(pi peer.AddrInfo) {
	if pi.ID == r.ID {
		return
	}
	r.log.Debugf("peer found: %s", pi.String())
}

func (r *relayer) subscribe() {
	for _, topic := range topicsSubs {
		r.log.Debugf("subscribing and relaying on topic %s", topic)
		t, err := r.topics.pubsub.Join(topic)
		if err != nil {
			r.log.Fatal(err)
		}
		_, err = t.Relay()
		if err != nil {
			r.log.Fatal(err)
		}
	}

}

func (r *relayer) handleStream(s network.Stream) {
	go r.receiveMessages(s.Conn().RemotePeer(), s)
}

func (r *relayer) receiveMessages(id peer.ID, reader io.Reader) {
	_ = r.processMessages(r.ctx, r.params.NetMagic, reader, func(message p2p.Message) error {
		cmd := message.Command()

		r.log.Tracef("processing message %s from peer %s", cmd, id)

		r.topics.topicsLock.Lock()
		defer r.topics.topicsLock.Unlock()
		topic, ok := r.topics.topics[cmd]
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

func (r *relayer) processMessages(ctx context.Context, net uint32, stream io.Reader, handler func(p2p.Message) error) error {
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

var idht *dht.IpfsDHT

var cmd = &cobra.Command{
	Use:   "relayer",
	Short: "Olympus DHT relayer",
	Long:  `Olympus DHT relayer`,
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.New(os.Stdin)
		log.WithColor()
		if debug {
			log.WithDebug()
		}

		ctx := context.Background()

		priv, err := loadPrivateKey()
		if err != nil {
			log.Fatal(err)
		}

		ds, err := dsbadger.NewDatastore("./peerstore", nil)
		if err != nil {
			log.Fatal(err)
		}

		ps, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
		if err != nil {
			log.Fatal(err)
		}

		listenAddress, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/" + port)
		if err != nil {
			log.Fatal(err)
		}

		h, err := libp2p.New(
			ctx,
			libp2p.ListenAddrs([]ma.Multiaddr{listenAddress}...),
			libp2p.Identity(priv),
			libp2p.Peerstore(ps),
			libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
				idht, err = dht.New(ctx, h, dht.Mode(dht.ModeAuto))
				return idht, err
			}),
			libp2p.ConnectionManager(connmgr.NewConnManager(
				100,
				400,
				time.Minute,
			)),
			libp2p.EnableAutoRelay(),
		)

		if err != nil {
			log.Fatal(err)
		}
		discoveryHandler := NewDiscoveryHandler(ctx)
		h.Network().Notify(discoveryHandler)

		syncHandler := NewSyncHandler(ctx)
		h.Network().Notify(syncHandler)

		addrs, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{
			ID:    h.ID(),
			Addrs: []ma.Multiaddr{listenAddress},
		})
		if err != nil {
			log.Fatal(err)
		}

		for _, a := range addrs {
			log.Infof("binding to address: %s", a)
		}

		err = idht.Bootstrap(ctx)
		if err != nil {
			log.Fatal(err)
		}

		r := discovery.NewRoutingDiscovery(idht)

		var netParams *params.ChainParams
		if net == "testnet" {
			netParams = &params.TestNet
		} else {
			netParams = &params.Mainnet
		}

		g, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			log.Fatal(err)
		}

		t := &topics{
			pubsub: g,
			topics: make(map[string]*pubsub.Topic),
		}

		relayer := relayer{
			ID:               h.ID(),
			log:              log,
			discovery:        r,
			dht:              idht,
			ctx:              ctx,
			params:           netParams,
			topics:           t,
			discoveryHandler: discoveryHandler,
			syncHandler:      syncHandler,
		}

		h.SetStreamHandler(params.DiscoveryProtocolID, relayer.handleStream)
		h.SetStreamHandler(params.SyncProtocolID, relayer.handleStream)

		if connect != "" {
			ma, err := ma.NewMultiaddr(connect)
			if err != nil {
				log.Error(err)
			}
			addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				log.Error(err)
			}
			err = h.Connect(ctx, *addrInfo)
			if err != nil {
				log.Error(err)
			}
		}

		relayer.subscribe()

		go relayer.findPeers()
		go relayer.advertise()

		<-ctx.Done()
	},
}

func loadPrivateKey() (crypto.PrivKey, error) {
	keyBytes, err := ioutil.ReadFile("./node_key.dat")
	if err != nil {
		return createPrivateKey()
	}

	key, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return createPrivateKey()
	}
	return key, nil
}

func createPrivateKey() (crypto.PrivKey, error) {
	_ = os.RemoveAll("./node_key.dat")

	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	keyBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile("./node_key.dat", keyBytes, 0700)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

func init() {
	cmd.Flags().BoolVar(&debug, "debug", false, "run the relayer with debug logger")
	cmd.Flags().StringVar(&port, "port", "25000", "port on which relayer will listen")
	cmd.Flags().StringVar(&net, "network", "testnet", "short name of the network to relay")
	cmd.Flags().StringVar(&connect, "connect", "", "string to connect to another relayer")
}

func main() {
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
