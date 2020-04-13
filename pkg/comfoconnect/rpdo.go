package comfoconnect

import (
	"bytes"
	"encoding/binary"
)

type RpdoTypeConverter interface {
	tofloat64() float64
}

type rpdoType struct {
	Description string
}

type RpdoType0 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType0) tofloat64() float64 {
	if r.rawValue == nil {
		return 0
	} else {
		return 1
	}
}

type RpdoType1 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType1) tofloat64() float64 {
	return float64(int(r.rawValue[0]))
}

type RpdoType2 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType2) tofloat64() float64 {
	return float64(binary.BigEndian.Uint16(r.rawValue))
}

type RpdoType3 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType3) tofloat64() float64 {
	return float64(binary.BigEndian.Uint32(r.rawValue))
}

type RpdoType6 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType6) tofloat64() float64 {
	var i int16
	_ = binary.Read(bytes.NewReader(r.rawValue), binary.BigEndian, &i)
	return float64(i)
}

func NewPpid(ppid uint32, message []byte) RpdoTypeConverter {
	switch ppid {
	case 16: return RpdoType1{rpdoType{"Unknown"}, message}
	case 33: return RpdoType1{rpdoType{"Unknown"}, message}
	case 37: return RpdoType1{rpdoType{"Unknown"}, message}
	case 49: return RpdoType1{rpdoType{"Operating mode1"}, message}
	case 53: return RpdoType1{rpdoType{"Unknown"}, message}
	case 56:
		return RpdoType1{rpdoType{"Operating mode2"}, message}
	case 65:
		return RpdoType1{rpdoType{"Fans: Fan speed setting"}, message}
	case 66: return RpdoType1{rpdoType{"Unknown"}, message}
	case 67: return RpdoType1{rpdoType{"Unknown"}, message}
	case 70: return RpdoType1{rpdoType{"Unknown"}, message}
	case 71: return RpdoType1{rpdoType{"Unknown"}, message}
	case 81:
		return RpdoType3{rpdoType{"General: Countdown until next fan speed change"}, message}
	case 82: return RpdoType3{rpdoType{"Unknown"}, message}
	case 85: return RpdoType3{rpdoType{"Unknown"}, message}
	case 86: return RpdoType3{rpdoType{"Unknown"}, message}
	case 87: return RpdoType3{rpdoType{"Unknown"}, message}
	case 117:
		return RpdoType1{rpdoType{"Fans: Exhaust fan duty"}, message}
	case 118:
		return RpdoType1{rpdoType{"Fans: Supply fan duty"}, message}
	case 119:
		return RpdoType2{rpdoType{"Fans: Exhaust fan flow"}, message}
	case 120:
		return RpdoType2{rpdoType{"Fans: Supply fan flow"}, message}
	case 121:
		return RpdoType2{rpdoType{"Fans: Exhaust fan speed"}, message}
	case 122:
		return RpdoType2{rpdoType{"Fans: Supply fan speed"}, message}
	case 128:
		return RpdoType2{rpdoType{"Power Consumption: Current Ventilation"}, message}
	case 129: return RpdoType2{rpdoType{"Power Consumption: Total year-to-date"}, message}
	case 130: return RpdoType2{rpdoType{"Power Consumption: Total from start"}, message}
	case 144: return RpdoType2{rpdoType{"Preheater Power Consumption: Total year-to-date"}, message}
	case 145: return RpdoType2{rpdoType{"Preheater Power Consumption: Total from start"}, message}
	case 146: return RpdoType2{rpdoType{"Preheater Power Consumption: Current Ventilation"}, message}
	case 176: return RpdoType1{rpdoType{"Unknown"}, message}
	case 192: return RpdoType2{rpdoType{"Days left before filters must be replaced"}, message}
	case 208: return RpdoType1{rpdoType{"Unknown temperature"}, message}
	case 209: return RpdoType6{rpdoType{"Current RMOT"}, message}
	case 210: return RpdoType0{rpdoType{"Unknown"}, message}
	case 211: return RpdoType0{rpdoType{"Unknown"}, message}
	case 212: return RpdoType6{rpdoType{"Unknown"}, message}
	case 213: return RpdoType2{rpdoType{"Avoided Heating: Avoided actual"}, message}
	case 214: return RpdoType2{rpdoType{"Avoided Heating: Avoided year-to-date"}, message}
	case 215: return RpdoType2{rpdoType{"Avoided Heating: Avoided total"}, message}
	case 216: return RpdoType2{rpdoType{"Avoided Cooling: Avoided actual"}, message}
	case 217: return RpdoType2{rpdoType{"Avoided Cooling: Avoided year-to-date"}, message}
	case 218: return RpdoType2{rpdoType{"Avoided Cooling: Avoided total"}, message}
	case 219: return RpdoType2{rpdoType{"Unknown"}, message}
	case 221: return RpdoType6{rpdoType{"Temperature & Humidity: Supply Air"}, message}
	case 224: return RpdoType1{rpdoType{"Unknown"}, message}
	case 225: return RpdoType1{rpdoType{"Unknown"}, message}
	case 226: return RpdoType1{rpdoType{"Unknown"}, message}
	case 227: return RpdoType1{rpdoType{"Bypass state"}, message}
	case 228: return RpdoType1{rpdoType{"Unknown Frost Protection Unbalance"}, message}
	case 274: return RpdoType6{rpdoType{"Temperature & Humidity: Extract Air"}, message}
	case 275: return RpdoType6{rpdoType{"Temperature & Humidity: Exhaust Air"}, message}
	case 276: return RpdoType6{rpdoType{"Temperature & Humidity: Outdoor Air"}, message}
	case 278: return RpdoType6{rpdoType{"PostHeaterTempBefore"}, message}
	case 290: return RpdoType1{rpdoType{"Temperature & Humidity: Extract Air"}, message}
	case 291: return RpdoType1{rpdoType{"Temperature & Humidity: Exhaust Air"}, message}
	case 292: return RpdoType1{rpdoType{"Temperature & Humidity: Outdoor Air"}, message}
	case 294: return RpdoType1{rpdoType{"Temperature & Humidity: Supply Air"}, message}
	case 321: return RpdoType2{rpdoType{"Unknown"}, message}
	case 325: return RpdoType2{rpdoType{"Unknown"}, message}
	case 337: return RpdoType3{rpdoType{"Unknown"}, message}
	case 338: return RpdoType3{rpdoType{"Unknown"}, message}
	case 341: return RpdoType3{rpdoType{"Unknown"}, message}
	case 369: return RpdoType1{rpdoType{"Unknown"}, message}
	case 370: return RpdoType1{rpdoType{"Unknown"}, message}
	case 371: return RpdoType1{rpdoType{"Unknown"}, message}
	case 372: return RpdoType1{rpdoType{"Unknown"}, message}
	case 384: return RpdoType6{rpdoType{"Unknown"}, message}
	case 386: return RpdoType0{rpdoType{"Unknown"}, message}
	case 400: return RpdoType6{rpdoType{"Unknown"}, message}
	case 401: return RpdoType1{rpdoType{"Unknown"}, message}
	case 402: return RpdoType0{rpdoType{"Unknown Post Heater Present"}, message}
	case 416: return RpdoType6{rpdoType{"unknown Outdoor air temperature"}, message}
	case 417: return RpdoType6{rpdoType{"unknown GHE Ground temperature"}, message}
	case 418: return RpdoType1{rpdoType{"unknown GHE State"}, message}
	case 419: return RpdoType0{rpdoType{"unknown GHE Present"}, message}
	case 785: return RpdoType0{rpdoType{"ComfoCoolCompressor State"}, message}
	default:
		return nil
	}
}
