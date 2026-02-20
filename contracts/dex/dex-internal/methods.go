package dexinternal

import "dex/sdk"

func (me *MaybeEnv) UseEnv() *sdk.Env {
	if me.env == nil {
		env := sdk.GetEnv()
		me.env = &env
	}
	return me.env
}
