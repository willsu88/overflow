package overflow

import (
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/sema"
	"golang.org/x/exp/slices"
)

func prepareOverflowArgument(parameterList []*ast.Parameter, inputArgs map[string]interface{}) (OverflowArgumentList, error) {
	argumentNotPresent := []string{} // only used to check that argument is not present, either extract to function or directly return nil
	argumentNames := []string{}
	args := OverflowArgumentList{}
	for _, parameter := range parameterList {
		parameterName := parameter.Identifier.Identifier
		value, ok := inputArgs[parameterName]
		if !ok {
			argumentNotPresent = append(argumentNotPresent, parameterName)
		} else {
			argumentNames = append(argumentNames, parameterName)
			args = append(args, OverflowArgument{
				Name:  parameterName,
				Value: value,
				Type:  parameter.TypeAnnotation.Type, //TypeAnnotion.Type is of `json:"AnnotatedType"`
			})
		}
	}

	if len(argumentNotPresent) > 0 {
		err := fmt.Errorf("the interaction is missing %v", argumentNotPresent)
		return nil, err
	}

	redundantArgument := []string{} // only used to check redudant argument.
	for inputKey := range inputArgs {
		// If your IDE complains about this it is wrong, this is 1.18 generics not suported anywhere
		if !slices.Contains(argumentNames, inputKey) {
			redundantArgument = append(redundantArgument, inputKey)
		}
	}

	if len(redundantArgument) > 0 {
		err := fmt.Errorf("the interaction has the following extra arguments %v", redundantArgument)
		return nil, err
	}
	return args, nil
}

func getParameterList(program *ast.Program) []*ast.Parameter {
	var parameterList []*ast.Parameter
	functionDeclaration := sema.FunctionEntryPointDeclaration(program)
	if functionDeclaration != nil {
		if functionDeclaration.ParameterList != nil {
			parameterList = functionDeclaration.ParameterList.Parameters
		}
	}

	transactionDeclaration := program.TransactionDeclarations()
	if len(transactionDeclaration) == 1 {
		if transactionDeclaration[0].ParameterList != nil {
			parameterList = transactionDeclaration[0].ParameterList.Parameters
		}
	}
	return parameterList
}
