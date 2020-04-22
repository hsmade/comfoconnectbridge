package comfoconnect

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/sirupsen/logrus"
)

type RpdoTypeConverter interface {
	Tofloat64() float64
	GetID() string
	GetDescription() string
}

type rpdoType struct {
	ID string
	Description string
}

func (r rpdoType) GetDescription() string {
	return r.Description
}

func (r rpdoType) GetID() string {
	return r.ID
}

type RpdoType0 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType0) Tofloat64() float64 {
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

func (r RpdoType1) Tofloat64() float64 {
	return float64(int(r.rawValue[0]))
}

type RpdoType2 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType2) Tofloat64() float64 {
	return float64(binary.BigEndian.Uint16(r.rawValue))
}

type RpdoType3 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType3) Tofloat64() float64 {
	return float64(binary.BigEndian.Uint32(r.rawValue))
}

type RpdoType6 struct {
	rpdoType
	rawValue []byte
}

func (r RpdoType6) Tofloat64() float64 {
	var i int16
	_ = binary.Read(bytes.NewReader(r.rawValue), binary.BigEndian, &i)
	return float64(i)
}

func NewPpid(ppid uint32, data []byte) RpdoTypeConverter {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "NewPpid",
		"ppid": ppid,
	})

	switch ppid {
	case 16:
		return RpdoType1{rpdoType{string(ppid), "Unknown"}, data}
	case 33:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 37:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 49:
		return RpdoType1{rpdoType{string(ppid),"Operating mode1"}, data}
	case 53:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 56:
		return RpdoType1{rpdoType{string(ppid),"Operating mode2"}, data}
	case 65:
		return RpdoType1{rpdoType{string(ppid),"Fans: Fan speed setting"}, data}
	case 66:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 67:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 70:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 71:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 81:
		return RpdoType3{rpdoType{string(ppid),"General: Countdown until next fan speed change"}, data}
	case 82:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 85:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 86:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 87:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 117:
		return RpdoType1{rpdoType{string(ppid),"Fans: Exhaust fan duty"}, data}
	case 118:
		return RpdoType1{rpdoType{string(ppid),"Fans: Supply fan duty"}, data}
	case 119:
		return RpdoType2{rpdoType{string(ppid),"Fans: Exhaust fan flow"}, data}
	case 120:
		return RpdoType2{rpdoType{string(ppid),"Fans: Supply fan flow"}, data}
	case 121:
		return RpdoType2{rpdoType{string(ppid),"Fans: Exhaust fan speed"}, data}
	case 122:
		return RpdoType2{rpdoType{string(ppid),"Fans: Supply fan speed"}, data}
	case 128:
		return RpdoType2{rpdoType{string(ppid),"Power Consumption: Current Ventilation"}, data}
	case 129:
		return RpdoType2{rpdoType{string(ppid),"Power Consumption: Total year-to-date"}, data}
	case 130:
		return RpdoType2{rpdoType{string(ppid),"Power Consumption: Total from start"}, data}
	case 144:
		return RpdoType2{rpdoType{string(ppid),"Preheater Power Consumption: Total year-to-date"}, data}
	case 145:
		return RpdoType2{rpdoType{string(ppid),"Preheater Power Consumption: Total from start"}, data}
	case 146:
		return RpdoType2{rpdoType{string(ppid),"Preheater Power Consumption: Current Ventilation"}, data}
	case 176:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 192:
		return RpdoType2{rpdoType{string(ppid),"Days left before filters must be replaced"}, data}
	case 208:
		return RpdoType1{rpdoType{string(ppid),"Unknown temperature"}, data}
	case 209:
		return RpdoType6{rpdoType{string(ppid),"Current RMOT"}, data}
	case 210:
		return RpdoType0{rpdoType{string(ppid),"Unknown"}, data}
	case 211:
		return RpdoType0{rpdoType{string(ppid),"Unknown"}, data}
	case 212:
		return RpdoType6{rpdoType{string(ppid),"Unknown"}, data}
	case 213:
		return RpdoType2{rpdoType{string(ppid),"Avoided Heating: Avoided actual"}, data}
	case 214:
		return RpdoType2{rpdoType{string(ppid),"Avoided Heating: Avoided year-to-date"}, data}
	case 215:
		return RpdoType2{rpdoType{string(ppid),"Avoided Heating: Avoided total"}, data}
	case 216:
		return RpdoType2{rpdoType{string(ppid),"Avoided Cooling: Avoided actual"}, data}
	case 217:
		return RpdoType2{rpdoType{string(ppid),"Avoided Cooling: Avoided year-to-date"}, data}
	case 218:
		return RpdoType2{rpdoType{string(ppid),"Avoided Cooling: Avoided total"}, data}
	case 219:
		return RpdoType2{rpdoType{string(ppid),"Unknown"}, data}
	case 221:
		return RpdoType6{rpdoType{string(ppid),"Temperature & Humidity: Supply Air"}, data}
	case 224:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 225:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 226:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 227:
		return RpdoType1{rpdoType{string(ppid),"Bypass state"}, data}
	case 228:
		return RpdoType1{rpdoType{string(ppid),"Unknown Frost Protection Unbalance"}, data}
	case 274:
		return RpdoType6{rpdoType{string(ppid),"Temperature & Humidity: Extract Air"}, data}
	case 275:
		return RpdoType6{rpdoType{string(ppid),"Temperature & Humidity: Exhaust Air"}, data}
	case 276:
		return RpdoType6{rpdoType{string(ppid),"Temperature & Humidity: Outdoor Air"}, data}
	case 278:
		return RpdoType6{rpdoType{string(ppid),"PostHeaterTempBefore"}, data}
	case 290:
		return RpdoType1{rpdoType{string(ppid),"Temperature & Humidity: Extract Air"}, data}
	case 291:
		return RpdoType1{rpdoType{string(ppid),"Temperature & Humidity: Exhaust Air"}, data}
	case 292:
		return RpdoType1{rpdoType{string(ppid),"Temperature & Humidity: Outdoor Air"}, data}
	case 294:
		return RpdoType1{rpdoType{string(ppid),"Temperature & Humidity: Supply Air"}, data}
	case 321:
		return RpdoType2{rpdoType{string(ppid),"Unknown"}, data}
	case 325:
		return RpdoType2{rpdoType{string(ppid),"Unknown"}, data}
	case 337:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 338:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 341:
		return RpdoType3{rpdoType{string(ppid),"Unknown"}, data}
	case 369:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 370:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 371:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 372:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 384:
		return RpdoType6{rpdoType{string(ppid),"Unknown"}, data}
	case 386:
		return RpdoType0{rpdoType{string(ppid),"Unknown"}, data}
	case 400:
		return RpdoType6{rpdoType{string(ppid),"Unknown"}, data}
	case 401:
		return RpdoType1{rpdoType{string(ppid),"Unknown"}, data}
	case 402:
		return RpdoType0{rpdoType{string(ppid),"Unknown Post Heater Present"}, data}
	case 416:
		return RpdoType6{rpdoType{string(ppid),"unknown Outdoor air temperature"}, data}
	case 417:
		return RpdoType6{rpdoType{string(ppid),"unknown GHE Ground temperature"}, data}
	case 418:
		return RpdoType1{rpdoType{string(ppid),"unknown GHE State"}, data}
	case 419:
		return RpdoType0{rpdoType{string(ppid),"unknown GHE Present"}, data}
	case 785:
		return RpdoType0{rpdoType{string(ppid),"ComfoCoolCompressor State"}, data}
	default:
		log.Errorf(fmt.Sprintf("unable to decode Rpdo with ppid: %d", ppid))
		return RpdoType6{rpdoType{string(ppid),"unknown"}, data}
	}
}
