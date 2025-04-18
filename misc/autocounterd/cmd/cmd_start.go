package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/commands"
)

type startCfg struct {
	rpcURL            string
	chainID           string
	mnemonic          string
	realmPath         string
	incrementInterval time.Duration
}

func (cfg *startCfg) Validate() error {
	switch {
	case cfg.rpcURL == "":
		return fmt.Errorf("rpc url cannot be empty")
	case cfg.chainID == "":
		return fmt.Errorf("chainID cannot be empty")
	case cfg.mnemonic == "":
		return fmt.Errorf("mnemonic cannot be empty")
	case cfg.realmPath == "":
		return fmt.Errorf("realmPath cannot be empty")
	case cfg.incrementInterval == 0:
		return fmt.Errorf("interval cannot be empty")
	}

	return nil
}

func NewStartCmd(io commands.IO) *commands.Command {
	cfg := &startCfg{}

	return commands.NewCommand(
		commands.Metadata{
			Name:       "start",
			ShortUsage: "start [flags]",
			ShortHelp:  "Increments the counter in the specified realm at regular intervals",
		},
		cfg,
		func(_ context.Context, args []string) error {
			return execStart(cfg, args, io)
		},
	)
}

func (cfg *startCfg) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&cfg.rpcURL, "rpc", "127.0.0.1:26657", "rpc url endpoint")
	fs.StringVar(&cfg.chainID, "chain-id", "dev", "chain-id")
	fs.StringVar(&cfg.mnemonic, "mnemonic", "", "mnemonic")
	fs.StringVar(&cfg.realmPath, "realm", "gno.land/r/portal/counter", "realm path")
	fs.DurationVar(&cfg.incrementInterval, "interval", 15*time.Second, "Increment counter interval")
}

func execStart(cfg *startCfg, args []string, io commands.IO) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	signer, err := gnoclient.SignerFromBip39(cfg.mnemonic, cfg.chainID, "", uint32(0), uint32(0))
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}
	if err := signer.Validate(); err != nil {
		return fmt.Errorf("invalid signer: %w", err)
	}

	rpcClient, err := rpcclient.NewHTTPClient(cfg.rpcURL)
	if err != nil {
		return fmt.Errorf("failed to create RPC client: %w", err)
	}

	client := gnoclient.Client{
		Signer:    signer,
		RPCClient: rpcClient,
	}

	for {
		_, err := client.Call(
			gnoclient.BaseTxCfg{
				GasFee:    "10000000ugnot",
				GasWanted: 800000,
			},
			vm.MsgCall{
				PkgPath: cfg.realmPath,
				Func:    "Incr",
				Args:    nil,
			})

		if err != nil {
			fmt.Printf("[ERROR] Failed to call Incr on %s: %v\n", cfg.realmPath, err)
		} else {
			fmt.Println("[INFO] Counter incremented with success")
		}
		time.Sleep(cfg.incrementInterval)
	}
}
