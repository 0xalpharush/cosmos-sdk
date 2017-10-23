// Generated by: main
// TypeWriter: wrapper
// Directive: +gen on EyesTxInner

package eyes

import (
	"github.com/tendermint/go-wire/data"
)

// Auto-generated adapters for happily unmarshaling interfaces
// Apache License 2.0
// Copyright (c) 2017 Ethan Frey (ethan.frey@tendermint.com)

type EyesTx struct {
	EyesTxInner "json:\"unwrap\""
}

var EyesTxMapper = data.NewMapper(EyesTx{})

func (h EyesTx) MarshalJSON() ([]byte, error) {
	return EyesTxMapper.ToJSON(h.EyesTxInner)
}

func (h *EyesTx) UnmarshalJSON(data []byte) (err error) {
	parsed, err := EyesTxMapper.FromJSON(data)
	if err == nil && parsed != nil {
		h.EyesTxInner = parsed.(EyesTxInner)
	}
	return err
}

// Unwrap recovers the concrete interface safely (regardless of levels of embeds)
func (h EyesTx) Unwrap() EyesTxInner {
	hi := h.EyesTxInner
	for wrap, ok := hi.(EyesTx); ok; wrap, ok = hi.(EyesTx) {
		hi = wrap.EyesTxInner
	}
	return hi
}

func (h EyesTx) Empty() bool {
	return h.EyesTxInner == nil
}

/*** below are bindings for each implementation ***/

func init() {
	EyesTxMapper.RegisterImplementation(SetTx{}, "set", 0x1)
}

func (hi SetTx) Wrap() EyesTx {
	return EyesTx{hi}
}

func init() {
	EyesTxMapper.RegisterImplementation(RemoveTx{}, "remove", 0x2)
}

func (hi RemoveTx) Wrap() EyesTx {
	return EyesTx{hi}
}
