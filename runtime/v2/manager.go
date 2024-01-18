package runtime

import (
	"context"
	"errors"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdkmodule "github.com/cosmos/cosmos-sdk/types/module"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/server/v2/core/appmanager"
	"cosmossdk.io/server/v2/stf"
)

type MMv2 struct {
	*sdkmodule.Manager
}

// TODO refactor away from mm
func (m *MMv2) BeginBlock() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		for _, moduleName := range m.OrderBeginBlockers {
			if module, ok := m.Modules[moduleName].(appmodule.HasBeginBlocker); ok {
				if err := module.BeginBlock(ctx); err != nil {
					return fmt.Errorf("beginblocker of module %s failure: %w", module, err)
				}
			}
		}

		return nil
	}
}

// TODO refactor away from mm
func (m *MMv2) EndBlock() (endblock func(ctx context.Context) error, valupdate func(ctx context.Context) ([]appmanager.ValidatorUpdate, error)) {
	validatorUpdates := []abci.ValidatorUpdate{}

	endBlock := func(ctx context.Context) error {
		for _, moduleName := range m.OrderEndBlockers {
			if module, ok := m.Modules[moduleName].(appmodule.HasEndBlocker); ok {
				err := module.EndBlock(ctx)
				if err != nil {
					return err
				}
			} else if module, ok := m.Modules[moduleName].(sdkmodule.HasABCIEndBlock); ok {
				moduleValUpdates, err := module.EndBlock(ctx)
				if err != nil {
					return err
				}
				// use these validator updates if provided, the module manager assumes
				// only one module will update the validator set
				if len(moduleValUpdates) > 0 {
					if len(validatorUpdates) > 0 {
						return errors.New("validator end block updates already set by a previous module")
					}

					for _, updates := range moduleValUpdates {
						validatorUpdates = append(validatorUpdates, abci.ValidatorUpdate{PubKey: updates.PubKey, Power: updates.Power})
					}
				}
			} else {
				continue
			}
		}

		return nil
	}

	valUpdate := func(ctx context.Context) ([]appmanager.ValidatorUpdate, error) {
		valUpdates := make([]appmanager.ValidatorUpdate, len(validatorUpdates))
		for i, v := range validatorUpdates {
			valUpdates[i] = appmanager.ValidatorUpdate{
				PubKey: v.PubKey.GetSecp256K1(),
				Power:  v.Power,
			}
		}

		return valUpdates, nil
	}

	return endBlock, valUpdate
}

func (m *MMv2) RegisterMsgs(builder *stf.MsgRouterBuilder) error { // most important part of the PR to finish
	// for _, module := range m.Modules {
	// 	builder.RegisterHandler()
	// 	builder.RegisterPostHandler()
	// 	builder.RegisterPreHandler()
	// }

	return nil
}

// UpgradeBlocker is PreBlocker for server v2, it supports only the upgrade module
func (m *MMv2) UpgradeBlocker() func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		for _, moduleName := range m.OrderPreBlockers {
			if moduleName != "upgrade" {
				continue
			}

			if module, ok := m.Modules[moduleName].(interface {
				UpgradeBlocker() func(ctx context.Context) (bool, error)
			}); ok {
				return module.UpgradeBlocker()(ctx)
			}
		}

		return false, fmt.Errorf("no upgrade module found")
	}
}
