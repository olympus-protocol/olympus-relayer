package main

import (
	"context"
	dsbadger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olympus-protocol/ogen/pkg/logger"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var (
	debug bool
	port  string
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

		ds, err := dsbadger.NewDatastore("./peerstore", nil)
		if err != nil {
			log.Fatal(err)
		}

		ps, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
		if err != nil {
			log.Fatal(err)
		}

		connman := connmgr.NewConnManager(2, 64, time.Second*60)

		h, err := libp2p.New(
			ctx,
			libp2p.ListenAddrs([]ma.Multiaddr{listenAddress}...),
			libp2p.Identity(priv),
			libp2p.EnableRelay(circuit.OptActive, circuit.OptHop),
			libp2p.NATPortMap(),
			libp2p.Peerstore(ps),
			libp2p.ConnectionManager(connman),
		)
		if err != nil {
			log.Fatal(err)
		}

	},
}

func init() {
	cmd.Flags().BoolVar(&debug, "debug", false, "run the relayer with debug logger")
	cmd.Flags().StringVar(&port, "port", "25000", "port on which relayer will listen")
}

func main() {
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
