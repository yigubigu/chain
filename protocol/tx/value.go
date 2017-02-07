package tx

import "chain/protocol/bc"

type valueSource struct {
	Ref      EntryRef
	Value    bc.AssetAmount
	Position uint64 // zero unless Ref is a mux
}

type valueDestination struct {
	Ref      EntryRef
	Position uint64
}
