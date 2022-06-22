package v3

import (
	"fmt"

	"github.com/bjartek/overflow/overflow"
	"github.com/onflow/cadence"
	"github.com/pkg/errors"
)

func Args(args ...interface{}) func(ftb *overflow.FlowInteractionBuilder) {

	return func(ftb *overflow.FlowInteractionBuilder) {
		if len(args)%2 != 0 {
			ftb.Error = fmt.Errorf("Please send in an even number of string : interface{} pairs")
			return
		}
		var i = 0
		for i < len(args) {
			key := args[0]
			value, labelOk := key.(string)
			if !labelOk {
				ftb.Error = fmt.Errorf("even parameters in Args needs to be strings")
			}
			ftb.NamedArgs[value] = args[1]
			i = i + 2
		}
	}
}

func ArgsM(args map[string]interface{}) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		for key, value := range args {
			ftb.NamedArgs[key] = value
		}
	}
}

func Arg(name, value string) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		ftb.NamedArgs[name] = value
	}
}

func CArg(name string, value cadence.Value) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		ftb.NamedArgs[name] = value
	}
}

func Addresses(name string, value ...string) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		array := []cadence.Value{}

		for _, val := range value {
			account, err := ftb.Overflow.AccountE(val)
			if err != nil {
				address, err := HexToAddress(val)
				if err != nil {
					ftb.Error = errors.Wrap(err, fmt.Sprintf("%s is not an valid account name or an address", val))
					return
				}
				cadenceAddress := cadence.BytesToAddress(address.Bytes())
				array = append(array, cadenceAddress)
			} else {
				cadenceAddress := cadence.BytesToAddress(account.Address().Bytes())
				array = append(array, cadenceAddress)
			}
		}
		ftb.NamedArgs[name] = cadence.NewArray(array)
	}
}

func ProposeAs(proposer string) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		account, err := ftb.Overflow.AccountE(proposer)
		if err != nil {
			ftb.Error = err
			return
		}
		ftb.Proposer = account
	}
}

func ProposeAsServiceAccount() func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		key := ftb.Overflow.ServiceAccountName()
		account, _ := ftb.Overflow.State.Accounts().ByName(key)
		ftb.Proposer = account
	}
}

func SignProposeAndPayAs(signer string) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		account, err := ftb.Overflow.AccountE(signer)
		if err != nil {
			ftb.Error = err
			return
		}
		ftb.MainSigner = account
		ftb.Proposer = account
	}
}

func SignProposeAndPayAsServiceAccount() func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		key := ftb.Overflow.ServiceAccountName()
		account, _ := ftb.Overflow.State.Accounts().ByName(key)
		ftb.MainSigner = account
		ftb.Proposer = account
	}
}

func Gas(gas uint64) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		ftb.GasLimit = gas
	}
}

func PayloadSigner(signer ...string) func(ftb *overflow.FlowInteractionBuilder) {
	return func(ftb *overflow.FlowInteractionBuilder) {
		for _, signer := range signer {
			account, err := ftb.Overflow.AccountE(signer)
			if err != nil {
				ftb.Error = err
				return
			}
			ftb.PayloadSigners = append(ftb.PayloadSigners, account)
		}
	}
}
