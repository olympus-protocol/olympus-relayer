package main

import (
	"context"
	"crypto/rand"
	externalip "github.com/glendc/go-external-ip"
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
	"path"
	"time"
)

var (
	datadir   string
	debug     bool
	port      string
	netstring string
	logfile   bool
)

var cmd = &cobra.Command{
	Use:   "relayer",
	Short: "Olympus DHT relayer",
	Long:  `Olympus DHT relayer`,
	Run: func(cmd *cobra.Command, args []string) {

		if datadir == "" {
			panic("Set a datadir with --datadir")
		}

		_ = os.MkdirAll(datadir, 0700)

		var log logger.Logger
		if logfile {
			logFile, err := os.OpenFile(path.Join(datadir, "logger.log"), os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				panic(err)
			}
			log = logger.New(logFile)
		} else {
			log = logger.New(os.Stdin)
			log.WithColor()
		}

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

		consensus := externalip.DefaultConsensus(nil, nil)

		log.Info("Getting external IP")

		ip, err := consensus.ExternalIP()
		if err != nil {
			panic(err)
		}

		var listenAddress []ma.Multiaddr

		ipv6 := false
		if ip.To4() == nil {
			ipv6 = true
		}

		var maStr string
		if ipv6 {
			maStr = "/ip6/::/tcp/"
		} else {
			maStr = "/ip4/0.0.0.0/tcp/"
		}
		maIpv4, err := ma.NewMultiaddr(maStr + port)
		if err != nil {
			log.Fatal(err)
		}

		// TODO here we can add more IPv6
		listenAddress = append(listenAddress, maIpv4)

		h, err := libp2p.New(
			ctx,
			libp2p.ListenAddrs(listenAddress...),
			libp2p.Identity(priv),
			libp2p.Peerstore(ps),
			libp2p.NATPortMap(),
			libp2p.ConnectionManager(connmgr.NewConnManager(
				256,
				2048,
				time.Minute,
			)),
			libp2p.EnableRelay(relay.OptActive, relay.OptHop),
		)

		if err != nil {
			log.Fatal(err)
		}

		protocol := "ip4"
		if ipv6 {
			protocol = "ip6"
		}

		log.Infof("binding to address: %s", "/"+protocol+"/"+ip.String()+"/tcp/"+port+"/p2p/"+h.ID().String())

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
		switch netstring {
		case "testnet":
			netParams = &params.TestNet
		case "devnet":
			netParams = &params.DevNet
		default:
			netParams = &params.MainNet
		}

		relay := relayer.NewRelayer(ctx, h, log, r, d, netParams)

		for name, r := range netParams.Relayers {
			ma, err := ma.NewMultiaddr(r)
			if err != nil {
				continue
			}
			peerAddr, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				log.Error(err)
				continue
			}
			if peerAddr.ID == h.ID() {
				continue
			}
			log.Infof("Connecting to %s", name)
			err = h.Connect(ctx, *peerAddr)
			if err != nil {
				log.Error(err)
				continue
			}
		}

		go relay.FindPeers()
		go relay.Advertise()

		<-ctx.Done()
		log.Infof("Closing Olympus Relayer")
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
	cmd.Flags().StringVar(&netstring, "network", "testnet", "short name of the network to relay")
	cmd.Flags().StringVar(&datadir, "datadir", "", "directory to save the peerstore and the private key")
	cmd.Flags().BoolVar(&logfile, "logfile", false, "enable logging into a file")
}

func main() {
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
