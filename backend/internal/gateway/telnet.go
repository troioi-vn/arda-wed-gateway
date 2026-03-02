package gateway

const (
	telnetIAC  = byte(255)
	telnetWILL = byte(251)
	telnetWONT = byte(252)
	telnetDO   = byte(253)
	telnetDONT = byte(254)
	telnetSB   = byte(250)
	telnetSE   = byte(240)
)

const (
	telnetStateData = iota
	telnetStateIAC
	telnetStateSkipOption
	telnetStateSubnegotiation
	telnetStateSubnegotiationIAC
)

type TelnetFilter struct {
	state int
}

func (f *TelnetFilter) Filter(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	out := make([]byte, 0, len(data))
	for _, b := range data {
		switch f.state {
		case telnetStateData:
			if b == telnetIAC {
				f.state = telnetStateIAC
				continue
			}
			out = append(out, b)
		case telnetStateIAC:
			switch b {
			case telnetWILL, telnetWONT, telnetDO, telnetDONT:
				f.state = telnetStateSkipOption
			case telnetSB:
				f.state = telnetStateSubnegotiation
			case telnetIAC:
				out = append(out, b)
				f.state = telnetStateData
			default:
				f.state = telnetStateData
			}
		case telnetStateSkipOption:
			f.state = telnetStateData
		case telnetStateSubnegotiation:
			if b == telnetIAC {
				f.state = telnetStateSubnegotiationIAC
			}
		case telnetStateSubnegotiationIAC:
			if b == telnetSE {
				f.state = telnetStateData
				continue
			}
			if b != telnetIAC {
				f.state = telnetStateSubnegotiation
			}
		}
	}

	return out
}
