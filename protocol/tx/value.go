package tx

import "chain/protocol/bc"

type valueSource struct {
	Ref      bc.Hash
	Value    bc.AssetAmount
	Position uint64 // zero unless Ref is a mux
}
