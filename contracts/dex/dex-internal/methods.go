package dexinternal

import "dex/sdk"

func (me *MaybeEnv) UseEnv() *sdk.Env {
	if me.Env == nil {
		env := sdk.GetEnv()
		me.Env = &env
	}
	return me.Env
}
