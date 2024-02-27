package overflow

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bjartek/underflow"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/flow-go-sdk"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type FilterFunction func(OverflowTransaction) bool

type BlockResult struct {
	StartTime         time.Time
	Error             error
	SystemChunkEvents OverflowEvents
	Logger            *zap.Logger
	Block             flow.Block
	Transactions      []OverflowTransaction
	View              uint64
}

type Argument struct {
	Value interface{}
	Key   string
}

type OverflowTransaction struct {
	Error            error
	AuthorizerTypes  OverflowAuthorizers
	Stakeholders     map[string][]string
	Payer            string
	Id               string
	Status           string
	BlockId          string
	Authorizers      []string
	Arguments        []Argument
	Events           []OverflowEvent
	Imports          []Import
	Script           []byte
	ProposalKey      flow.ProposalKey
	Fee              float64
	TransactionIndex int
	GasLimit         uint64
	GasUsed          uint64
	ExecutionEffort  float64
}

func (o *OverflowState) CreateOverflowTransaction(blockId string, transactionResult flow.TransactionResult, transaction flow.Transaction, txIndex int) (*OverflowTransaction, error) {
	feeAmount := 0.0
	events, fee := o.ParseEvents(transactionResult.Events, "")
	feeRaw, ok := fee.Fields["amount"]
	if ok {
		feeAmount, ok = feeRaw.(float64)
		if !ok {
			return nil, fmt.Errorf("failed casting fee amount to float64")
		}
	}

	executionEffort, ok := fee.Fields["executionEffort"].(float64)
	gas := 0
	if ok {
		factor := 100000000
		gas = int(math.Round(executionEffort * float64(factor)))
	}

	status := transactionResult.Status.String()

	args := []Argument{}
	argInfo := declarationInfo(transaction.Script)
	for i := range transaction.Arguments {
		arg, err := transaction.Argument(i)
		if err != nil {
			status = fmt.Sprintf("%s failed getting argument at index %d", status, i)
		}
		var key string
		if len(argInfo.ParameterOrder) <= i {
			key = "invalid"
		} else {
			key = argInfo.ParameterOrder[i]
		}
		argStruct := Argument{
			Key:   key,
			Value: underflow.CadenceValueToInterfaceWithOption(arg, o.UnderflowOptions),
		}
		args = append(args, argStruct)
	}

	standardStakeholders := map[string][]string{}
	imports, err := GetAddressImports(transaction.Script)
	if err != nil {
		status = fmt.Sprintf("%s failed getting imports", status)
	}

	authorizers := []string{}
	for _, authorizer := range transaction.Authorizers {
		auth := fmt.Sprintf("0x%s", authorizer.Hex())
		authorizers = append(authorizers, auth)
		standardStakeholders[auth] = []string{"authorizer"}
	}

	payerRoles, ok := standardStakeholders[fmt.Sprintf("0x%s", transaction.Payer.Hex())]
	if !ok {
		standardStakeholders[fmt.Sprintf("0x%s", transaction.Payer.Hex())] = []string{"payer"}
	} else {
		payerRoles = append(payerRoles, "payer")
		standardStakeholders[fmt.Sprintf("0x%s", transaction.Payer.Hex())] = payerRoles
	}

	proposer, ok := standardStakeholders[fmt.Sprintf("0x%s", transaction.ProposalKey.Address.Hex())]
	if !ok {
		standardStakeholders[fmt.Sprintf("0x%s", transaction.ProposalKey.Address.Hex())] = []string{"proposer"}
	} else {
		proposer = append(proposer, "proposer")
		standardStakeholders[fmt.Sprintf("0x%s", transaction.ProposalKey.Address.Hex())] = proposer
	}

	eventsWithoutFees := events.FilterFees(feeAmount, fmt.Sprintf("0x%s", transaction.Payer.Hex()))

	eventList := []OverflowEvent{}
	for _, evList := range eventsWithoutFees {
		eventList = append(eventList, evList...)
	}

	return &OverflowTransaction{
		Id:               transactionResult.TransactionID.String(),
		TransactionIndex: txIndex,
		BlockId:          blockId,
		Status:           status,
		Events:           eventList,
		Stakeholders:     eventsWithoutFees.GetStakeholders(standardStakeholders),
		Imports:          imports,
		Error:            transactionResult.Error,
		Arguments:        args,
		Fee:              feeAmount,
		Script:           transaction.Script,
		Payer:            fmt.Sprintf("0x%s", transaction.Payer.String()),
		ProposalKey:      transaction.ProposalKey,
		GasLimit:         transaction.GasLimit,
		GasUsed:          uint64(gas),
		ExecutionEffort:  executionEffort,
		Authorizers:      authorizers,
		AuthorizerTypes:  argInfo.Authorizers,
	}, nil
}

func (o *OverflowState) GetOverflowTransactionById(ctx context.Context, id flow.Identifier) (*OverflowTransaction, error) {
	tx, txr, err := o.Flowkit.GetTransactionByID(ctx, id, false)
	if err != nil {
		return nil, err
	}
	txIndex := 0
	if len(txr.Events) > 0 {
		txIndex = txr.Events[0].TransactionIndex
	}
	return o.CreateOverflowTransaction(txr.BlockID.String(), *txr, *tx, txIndex)
}

func (o *OverflowState) GetTransactionById(ctx context.Context, id flow.Identifier) (*flow.Transaction, error) {
	tx, _, err := o.Flowkit.GetTransactionByID(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// this is get from block, needs to return system chunk information
func (o *OverflowState) GetOverflowTransactionsForBlockId(ctx context.Context, id flow.Identifier, logg *zap.Logger) ([]OverflowTransaction, OverflowEvents, error) {
	// sometimes this will become too complex.

	//if we get this error
	//* rpc error: code = ResourceExhausted desc = grpc: trying to send message larger than max (22072361 vs. 20971520)
	//we have to fetch the block again with transaction ids.
	//in parallel loop over them and run GetStatus and create the transactions that way.

	tx, txR, err := o.Flowkit.GetTransactionsByBlockID(ctx, id)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting transaction results")
	}

	logg.Debug("Fetched tx", zap.String("blockId", id.String()), zap.Int("tx", len(tx)), zap.Int("txR", len(txR)))
	var systemChunkEvents OverflowEvents
	totalTxR := len(txR)
	result := lo.FlatMap(txR, func(rp *flow.TransactionResult, i int) []OverflowTransaction {
		r := *rp
		isLatestResult := totalTxR == i+1
		// on network emulator we never have system chunk transactions it looks like
		if isLatestResult && o.GetNetwork() != "emulator" {
			systemChunkEvents, _ = o.ParseEvents(r.Events, fmt.Sprintf("%d-", r.BlockHeight))
			if len(systemChunkEvents) > 0 {
				logg.Debug("We have system chunk events", zap.Int("systemEvents", len(systemChunkEvents)))
			}
			return []OverflowTransaction{}
		}
		t := *tx[i]

		// for some reason we get epoch heartbeat
		if len(t.EnvelopeSignatures) == 0 {
			return []OverflowTransaction{}
		}

		ot, err := o.CreateOverflowTransaction(id.String(), r, t, i)
		if err != nil {
			panic(err)
		}
		return []OverflowTransaction{*ot}
	})

	return result, systemChunkEvents, nil
}

func (o *OverflowState) GetBlockResult(ctx context.Context, height uint64, logg *zap.Logger) (*BlockResult, error) {
	logg.Debug("first")
	start := time.Now()
	block, err := o.GetBlockAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	logg.Debug("second")
	tx, systemChunkEvents, err := o.GetOverflowTransactionsForBlockId(ctx, block.ID, logg)
	if err != nil {
		return nil, err
	}

	return &BlockResult{Block: *block, Transactions: tx, SystemChunkEvents: systemChunkEvents, Logger: logg, View: 0, StartTime: start}, nil
}

// This code is beta
func (o *OverflowState) StreamTransactions(ctx context.Context, poll time.Duration, height uint64, logger *zap.Logger, channel chan<- BlockResult) error {
	latestKnownBlock, err := o.GetLatestBlock(ctx)
	if err != nil {
		return err
	}
	logger.Info("latest block is", zap.Uint64("height", latestKnownBlock.Height))

	sleep := poll
	for {
		select {
		case <-time.After(sleep):

			start := time.Now()
			sleep = poll
			nextBlockToProcess := height + 1
			if height == uint64(0) {
				nextBlockToProcess = latestKnownBlock.Height
				height = latestKnownBlock.Height
			}
			logg := logger.With(zap.Uint64("height", nextBlockToProcess), zap.Uint64("latestKnownBlock", latestKnownBlock.Height))
			logg.Debug("tick")

			var block *flow.Block
			if nextBlockToProcess < latestKnownBlock.Height {
				logg.Debug("next block is smaller then latest known block")
				// we are still processing historical blocks
				block, err = o.GetBlockAtHeight(ctx, nextBlockToProcess)
				if err != nil {
					logg.Info("error fetching old block", zap.Error(err))
					continue
				}
			} else if nextBlockToProcess != latestKnownBlock.Height {
				logg.Debug("next block is not equal to latest block")
				block, err = o.GetLatestBlock(ctx)
				if err != nil {
					logg.Info("error fetching latest block, retrying", zap.Error(err))
					continue
				}

				if block == nil || block.Height == latestKnownBlock.Height {
					continue
				}
				latestKnownBlock = block
				// we just continue the next iteration in the loop here
				sleep = time.Millisecond
				// the reason we just cannot process here is that the latestblock might not be the next block we should process
				continue
			} else {
				block = latestKnownBlock
			}
			readDur := time.Since(start)
			logg.Info("block read", zap.Any("block", block.Height), zap.Any("latestBlock", latestKnownBlock.Height), zap.Any("readDur", readDur.Seconds()))
			tx, systemChunkEvents, err := o.GetOverflowTransactionsForBlockId(ctx, block.ID, logg)
			logg.Debug("fetched transactions", zap.Int("tx", len(tx)))
			if err != nil {
				logg.Debug("getting transaction", zap.Error(err))
				if strings.Contains(err.Error(), "could not retrieve collection: key not found") {
					continue
				}

				select {
				case channel <- BlockResult{Block: *block, SystemChunkEvents: systemChunkEvents, Error: errors.Wrap(err, "getting transactions"), Logger: logg, View: 0, StartTime: start}:
					height = nextBlockToProcess
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}
			logg = logg.With(zap.Int("tx", len(tx)))
			select {
			case channel <- BlockResult{Block: *block, Transactions: tx, SystemChunkEvents: systemChunkEvents, Logger: logg, View: 0, StartTime: start}:
				height = nextBlockToProcess
			case <-ctx.Done():
				return ctx.Err()
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func GetAddressImports(code []byte) ([]Import, error) {
	deps := []Import{}
	program, err := parser.ParseProgram(nil, code, parser.Config{})
	if err != nil {
		return deps, err
	}

	for _, imp := range program.ImportDeclarations() {
		address, isAddressImport := imp.Location.(common.AddressLocation)
		if isAddressImport {
			for _, id := range imp.Identifiers {
				deps = append(deps, Import{
					Address: fmt.Sprintf("0x%s", address.Address.Hex()),
					Name:    id.Identifier,
				})
			}
		}
	}
	return deps, nil
}

type Import struct {
	Address string
	Name    string
}

func (i Import) Identifier() string {
	return fmt.Sprintf("A.%s.%s", strings.TrimPrefix(i.Address, "0x"), i.Name)
}
