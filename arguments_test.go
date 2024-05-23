package overflow

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareArguments(t *testing.T) {
	// prepareOverflowArgument(parameterList []*ast.Parameter, inputArgs map[string]interface{}

	t.Run("should fail if redundant arguments", func(t *testing.T) {
		parameterList := []*ast.Parameter{}
		inputArgs := map[string]interface{}{
			"something": "William",
		}

		args, err := prepareOverflowArgument(parameterList, inputArgs)

		assert.Nil(t, args)
		assert.Error(t, err)
		// todo: need to check what error is
	})

	t.Run("should fail if argument not present", func(t *testing.T) {
		parameterList := []*ast.Parameter{
			ast.NewParameter(nil, "something", ast.Identifier{Identifier: "name"}, &ast.TypeAnnotation{Type: &ast.NominalType{Identifier: ast.Identifier{Identifier: "string"}}}, nil, ast.Position{}),
		}
		inputArgs := map[string]interface{}{}

		args, err := prepareOverflowArgument(parameterList, inputArgs)

		assert.Nil(t, args)
		assert.EqualError(t, err, "the interaction is missing [name]")
	})

	t.Run("should succeed if one string argument", func(t *testing.T) {
		parameterList := []*ast.Parameter{
			ast.NewParameter(nil, "something", ast.Identifier{Identifier: "name"}, &ast.TypeAnnotation{Type: &ast.NominalType{Identifier: ast.Identifier{Identifier: "string"}}}, nil, ast.Position{}),
		}
		inputArgs := map[string]interface{}{
			"name": "William",
		}

		args, err := prepareOverflowArgument(parameterList, inputArgs)
		require.NoError(t, err)

		expectedArgs := OverflowArgumentList{{
			Name:  "name",
			Value: "William",
			Type:  parameterList[0].TypeAnnotation.Type, //TypeAnnotion.Type is of `json:"AnnotatedType"`
		}}

		assert.Equal(t, expectedArgs, args)
	})
}

func TestParseArguments(t *testing.T) {
	g, err := OverflowTesting()
	require.NoError(t, err)
	require.NotNil(t, g)
	t.Run("should succeed if one nil argument", func(t *testing.T) {
		inputArgs := map[string]interface{}{
			"name": nil,
		}

		args, argMap, err := g.parseArguments("test.cdc", []byte(`
		access(all) fun main(name: String?): AnyStruct {
			return nil
		}`), inputArgs)
		require.NoError(t, err)
		require.Equal(t, args[0], argMap["name"])
		require.Equal(t, cadence.Optional{Value: nil}, args[0])
	})

	t.Run("should succeed if one int argument", func(t *testing.T) {
		inputArgs := map[string]interface{}{
			"age": 25,
		}

		args, argMap, err := g.parseArguments("test.cdc", []byte(`
		access(all) fun main(age: Int): AnyStruct {
			return age
		}`), inputArgs)
		require.NoError(t, err)
		require.Equal(t, args[0], argMap["age"])
		require.Equal(t, cadence.NewInt(25), args[0])
	})
}
