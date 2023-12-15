package appmanager

import (
	"context"
	"time"
)

var runtimeIdentity Identity = []byte("app-manager")

type executionContext struct {
	context.Context
	store    BranchStore
	gasUsed  uint64
	gasLimit uint64
	events   []Event
	sender   Identity
}

type TxDecoder interface {
	Decode([]byte) (Tx, error)
}

type Tx interface {
	GetMessage() Type
	GetSender() Identity
	GetGasLimit() uint64
}

type Block struct {
	Height            uint64
	Time              time.Time
	Hash              []byte
	Txs               [][]byte
	ConsensusMessages []Type // <= proto.Message
}

type BlockResponse struct {
	BeginBlockEvents           []Event
	TxResults                  []TxResult
	EndBlockEvents             []Event
	ConsensusMessagesResponses []Type
}

type TxResult struct {
	Events  []Event
	GasUsed uint64

	Resp  Type
	Error error
}

// STFAppManager is a struct that manages the state transition component of the app.
type STFAppManager struct {
	handleMsg   func(ctx context.Context, msg Type) (msgResp Type, err error)
	handleQuery func(ctx context.Context, req Type) (resp Type, err error)

	doBeginBlock func(ctx context.Context) error
	doEndBlock   func(ctx context.Context) error

	doTxValidation func(ctx context.Context, tx Tx) error

	decodeTx func(txBytes []byte) (Tx, error)

	branch func(store ReadonlyStore) BranchStore // branch is a function that given a readonly store it returns a writable version of it.
}

// DeliverBlock is our state transition function.
// It takes a read only view of the state to apply the block to,
// executes the block and returns the block results and the new state.
func (s STFAppManager) DeliverBlock(ctx context.Context, block Block, state ReadonlyStore) (blockResult *BlockResponse, newState BranchStore, err error) {
	blockResult = new(BlockResponse)
	// creates a new branch store, from the readonly view of the state
	// that can be written to.
	newState = s.branch(state)
	// begin block
	beginBlockEvents, err := s.beginBlock(ctx, newState)
	if err != nil {
		return nil, nil, err
	}
	// execute txs
	txResults := make([]TxResult, len(block.Txs))
	for i, txBytes := range block.Txs {
		txResults[i] = s.deliverTx(ctx, newState, txBytes)
	}
	// end block
	endBlockEvents, err := s.endBlock(ctx, newState, block)
	if err != nil {
		return nil, nil, err
	}

	return &BlockResponse{
		BeginBlockEvents: beginBlockEvents,
		TxResults:        txResults,
		EndBlockEvents:   endBlockEvents,
	}, newState, nil
}

func (s STFAppManager) beginBlock(ctx context.Context, state BranchStore) (beginBlockEvents []Event, err error) {
	execCtx := s.makeContext(ctx, runtimeIdentity, state, 0) // TODO: gas limit
	err = s.doBeginBlock(execCtx)
	if err != nil {
		return nil, err
	}
	// apply state changes
	changes, err := execCtx.store.ChangeSets()
	if err != nil {
		return nil, err
	}
	return execCtx.events, state.ApplyChangeSets(changes)
}

func (s STFAppManager) deliverTx(ctx context.Context, state BranchStore, txBytes []byte) TxResult {
	tx, err := s.decodeTx(txBytes)
	if err != nil {
		return TxResult{
			Error: err,
		}
	}

	validateGas, validationEvents, err := s.validateTx(ctx, state, tx.GetGasLimit(), tx)
	if err != nil {
		return TxResult{
			Error: err,
		}
	}

	execResp, execGas, execEvents, err := s.execTx(ctx, state, tx.GetGasLimit()-validateGas, tx)
	if err != nil {
		return TxResult{
			Events:  validationEvents,
			GasUsed: validateGas + execGas,
			Error:   err,
		}
	}

	return TxResult{
		Events:  append(validationEvents, execEvents...),
		GasUsed: execGas + validateGas,
		Resp:    execResp,
		Error:   nil,
	}
}

// validateTx validates a transaction given the provided BranchStore and gas limit.
// If the validation is successful, state is committed
func (s STFAppManager) validateTx(ctx context.Context, store BranchStore, gasLimit uint64, tx Tx) (gasUsed uint64, events []Event, err error) {
	validateCtx := s.makeContext(ctx, tx.GetSender(), store, gasLimit)
	err = s.doTxValidation(ctx, tx)
	if err != nil {
		return 0, nil, nil
	}
	// all went fine we can commit to state.
	changeSets, err := validateCtx.store.ChangeSets()
	if err != nil {
		return 0, nil, err
	}
	err = store.ApplyChangeSets(changeSets)
	if err != nil {
		return 0, nil, err
	}
	return validateCtx.gasUsed, validateCtx.events, nil
}

func (s STFAppManager) execTx(ctx context.Context, store BranchStore, gasLimit uint64, tx Tx) (msgResp Type, gasUsed uint64, execEvents []Event, err error) {
	execCtx := s.makeContext(ctx, tx.GetSender(), store, gasLimit)
	msgResp, err = s.handleMsg(ctx, tx.GetMessage())
	if err != nil {
		return nil, 0, nil, err
	}
	// get state changes and save them to the parent store
	changeSets, err := execCtx.store.ChangeSets()
	if err != nil {
		return nil, 0, nil, err
	}
	err = store.ApplyChangeSets(changeSets)
	if err != nil {
		return nil, 0, nil, err
	}
	return msgResp, 0, execCtx.events, nil
}

func (s STFAppManager) endBlock(ctx context.Context, store BranchStore, block Block) (endBlockEvents []Event, err error) {
	execCtx := s.makeContext(ctx, runtimeIdentity, store, 0) // TODO: gas limit
	err = s.doBeginBlock(execCtx)
	if err != nil {
		return nil, err
	}
	// apply state changes
	changes, err := execCtx.store.ChangeSets()
	if err != nil {
		return nil, err
	}
	return execCtx.events, store.ApplyChangeSets(changes)
}

func (s STFAppManager) makeContext(
	ctx context.Context,
	sender Identity,
	store BranchStore,
	gasLimit uint64,
) *executionContext {
	return &executionContext{
		Context:  ctx,
		store:    store,
		gasUsed:  0,
		gasLimit: gasLimit,
		events:   make([]Event, 0),
		sender:   sender,
	}
}
