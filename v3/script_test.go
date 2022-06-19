package v3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScript(t *testing.T) {
	o, _ := OverflowTesting()

	t.Run("Run simple script interface", func(t *testing.T) {
		res := o.Script("test", Arg("account", "first")).GetAsInterface()
		assert.Equal(t, "0x01cf0e2f2f715450", res)
	})

	t.Run("Run simple script json", func(t *testing.T) {
		res := o.Script("test", Arg("account", "first")).GetAsJson()
		assert.Equal(t, `"0x01cf0e2f2f715450"`, res)
	})

	t.Run("Run simple script marshal", func(t *testing.T) {
		var res string
		err := o.Script("test", Arg("account", "first")).MarhalAs(&res)
		assert.NoError(t, err)
		assert.Equal(t, "0x01cf0e2f2f715450", res)
	})

	t.Run("compose a script", func(t *testing.T) {
		accountScript := o.ScriptFN(Arg("account", "first"))
		res := accountScript("test")
		assert.NoError(t, res.Err)
	})

	t.Run("create script with name", func(t *testing.T) {
		testScript := o.ScriptFileNameFN("test")
		res := testScript(Arg("account", "first"))
		assert.NoError(t, res.Err)
	})

}