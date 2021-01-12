package main

import (
	"context"
	"crypto/rand"
	"fmt"
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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sort"
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

		ips, err := retrieveIPAddrs()

		if err != nil {
			log.Fatal(err)
		}

		var mas []ma.Multiaddr
		for _, ip := range ips {
			ma, err := multiAddressBuilder(ip.String(), port)
			if err != nil {
				log.Fatal(err)
			}
			mas = append(mas, ma)
		}

		h, err := libp2p.New(
			ctx,
			libp2p.ListenAddrs(mas...),
			libp2p.Identity(priv),
			libp2p.Peerstore(ps),
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

		for _, ma := range mas {
			log.Infof("Binding to %s/p2p/%s", ma.String(), h.ID().String())
		}

		var netParams *params.ChainParams
		switch netstring {
		case "testnet":
			netParams = &params.TestNet
		default:
			netParams = &params.MainNet
		}

		var bootNodes []peer.AddrInfo
		for _, r := range netParams.Relayers {
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
			bootNodes = append(bootNodes, *peerAddr)
		}

		d, err := dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.ProtocolPrefix(params.ProtocolID(netParams.Name)), dht.BootstrapPeers(bootNodes...))
		if err != nil {
			log.Fatal(err)
		}

		err = d.Bootstrap(ctx)
		if err != nil {
			log.Fatal(err)
		}

		r := discovery.NewRoutingDiscovery(d)

		relay := relayer.NewRelayer(ctx, h, log, r, d, netParams)

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

func retrieveIPAddrs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ipAddrs []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ipAddrs = append(ipAddrs, ip)
		}
	}
	return SortAddresses(ipAddrs), nil
}

// SortAddresses sorts a set of addresses in the order of
// ipv4 -> ipv6.
func SortAddresses(ipAddrs []net.IP) []net.IP {
	sort.Slice(ipAddrs, func(i, j int) bool {
		return ipAddrs[i].To4() != nil && ipAddrs[j].To4() == nil
	})
	return ipAddrs
}

func multiAddressBuilder(ipAddr string, port string) (ma.Multiaddr, error) {
	parsedIP := net.ParseIP(ipAddr)
	if parsedIP.To4() == nil && parsedIP.To16() == nil {
		return nil, errors.Errorf("invalid ip address provided: %s", ipAddr)
	}
	if parsedIP.To4() != nil {
		return ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", ipAddr, port))
	}
	return ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/%s", ipAddr, port))
}
