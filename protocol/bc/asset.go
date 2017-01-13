package bc

import (
	"database/sql/driver"
	"io"

	"chain/crypto/sha3pool"
	"chain/encoding/blockchain"
)

const assetVersion = 1

// AssetID is the Hash256 of the issuance script for the asset and the
// initial block of the chain where it appears.
type AssetID [32]byte

func (a AssetID) String() string                { return Hash(a).String() }
func (a AssetID) MarshalText() ([]byte, error)  { return Hash(a).MarshalText() }
func (a *AssetID) UnmarshalText(b []byte) error { return (*Hash)(a).UnmarshalText(b) }
func (a *AssetID) UnmarshalJSON(b []byte) error { return (*Hash)(a).UnmarshalJSON(b) }
func (a AssetID) Value() (driver.Value, error)  { return Hash(a).Value() }
func (a *AssetID) Scan(b interface{}) error     { return (*Hash)(a).Scan(b) }

// ComputeAssetID computes the asset ID of the asset defined by
// the given issuance program and initial block hash.
func ComputeAssetID(issuanceProgram []byte, initialHash [32]byte, vmVersion uint64, assetDefinitionHash Hash) (assetID AssetID) {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)
	h.Write(initialHash[:])
	blockchain.WriteVarint63(h, vmVersion)
	blockchain.WriteVarstr31(h, issuanceProgram) // TODO(bobg): check and return error
	h.Write(assetDefinitionHash[:])
	h.Read(assetID[:])
	return assetID
}

// AssetAmount combines an asset id and an amount.
type AssetAmount struct {
	AssetID AssetID `json:"asset_id"`
	Amount  uint64  `json:"amount"`
}

// assumes r has sticky errors
func (a *AssetAmount) readFrom(r io.Reader) (int, error) {
	return blockchain.Read(r, a)
}

func (a *AssetAmount) writeTo(w io.Writer) (int64, error) {
	n, err := blockchain.Write(w, 0, false, a)
	return int64(n), err
}
