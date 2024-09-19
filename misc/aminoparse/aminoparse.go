package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/std"

	_ "github.com/gnolang/gno/gno.land/pkg/sdk/vm"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var blockData struct {
		Result struct {
			Block struct {
				Data struct {
					Txs [][]byte `json:"txs"`
				} `json:"data"`
			} `json:"block"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &blockData); err != nil {
		panic(err)
	}
	for _, txBytes := range blockData.Result.Block.Data.Txs {
		var tx std.Tx
		amino.MustUnmarshal(txBytes, &tx)
		fmt.Printf("%+v\n", tx)
	}
}
