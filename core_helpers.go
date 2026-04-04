package infra

import core "dappco.re/go/core"

var localFS = (&core.Fs{}).NewUnrestricted()

func coreResultErr(r core.Result, op string) error {
	if r.OK {
		return nil
	}
	if err, ok := r.Value.(error); ok && err != nil {
		return err
	}
	if r.Value == nil {
		return core.E(op, "unexpected empty core result", nil)
	}
	return core.E(op, core.Sprint(r.Value), nil)
}
