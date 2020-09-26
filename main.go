package main

import (
	"context"
	"crypto/rand"
	dsbadger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	relay "github.com/libp2p/go-libp2p-circuit"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/olympus-protocol/ogen/pkg/params"
	"github.com/olympus-protocol/olympus-relayer/relayer"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"time"
)

var (
	debug   bool
	port    string
	net     string
	connect string
)

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
			libp2p.NATPortMap(),
			libp2p.ConnectionManager(connmgr.NewConnManager(
				1,
				2048,
				time.Minute,
			)),
			libp2p.EnableRelay(relay.OptActive, relay.OptHop),
		)

		if err != nil {
			log.Fatal(err)
		}

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

		d, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
		if err != nil {
			log.Fatal(err)
		}

		err = d.Bootstrap(ctx)
		if err != nil {
			log.Fatal(err)
		}

		r := discovery.NewRoutingDiscovery(d)

		var netParams *params.ChainParams
		if net == "testnet" {
			netParams = &params.TestNet
		} else {
			netParams = &params.Mainnet
		}

		relayer := relayer.NewRelayer(ctx, h, log, r, d, netParams)

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

		relayer.Subscribe()

		go relayer.FindPeers()
		go relayer.Advertise()

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
