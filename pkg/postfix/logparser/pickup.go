package parser

import (
	"gitlab.com/lightmeter/controlcenter/pkg/postfix/logparser/rawparser"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
)

func init() {
	registerHandler(rawparser.PayloadTypePickup, convertPickup)
}

type Pickup struct {
	Queue  string
	Uid    int
	Sender string
}

func (Pickup) isPayload() {
	// required by interface Payload
}

func convertPickup(r rawparser.RawPayload) (Payload, error) {
	p := r.Pickup

	uid, err := atoi(p.Uid)
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	return Pickup{
		Queue:  string(p.Queue),
		Uid:    uid,
		Sender: string(p.Sender),
	}, nil
}
