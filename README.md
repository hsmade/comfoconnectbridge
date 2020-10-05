# comfoconnectbridge
Tries to be a transparent bridge for comfoconnect while gathering metrics and accepting commands over API

## Proto file
The proto file was copied from https://github.com/michaelarnauts/comfoconnect/blob/master/protobuf/zehnder.proto

## Current status
The [client](cmd/client) and [dumbproxy](cmd/dumbproxy) work, but the actual [proxy](cmd/proxy) doesn't always work.
* The client acts as a client, trying to fetch regular updates and exposing them through prometheus.
* The dumbproxy acts as a proxy, but needs an actual client to do requests, and will then expose metrics for the values found, through prometheus
* The proxy is supposed to be a combination of the two.


## metrics
Example output:

```# HELP comfoconnect_pdo_value Value for the different PDOs, as they're seen by the proxy
# TYPE comfoconnect_pdo_value gauge
comfoconnect_pdo_value{ID="117",description="Fans: Exhaust fan duty"} 46
comfoconnect_pdo_value{ID="118",description="Fans: Supply fan duty"} 44
comfoconnect_pdo_value{ID="119",description="Fans: Exhaust fan flow"} 185
comfoconnect_pdo_value{ID="120",description="Fans: Supply fan flow"} 175
comfoconnect_pdo_value{ID="121",description="Fans: Exhaust fan speed"} 1730
comfoconnect_pdo_value{ID="122",description="Fans: Supply fan speed"} 1629
comfoconnect_pdo_value{ID="128",description="Power Consumption: Current Ventilation"} 33
comfoconnect_pdo_value{ID="129",description="Power Consumption: Total year-to-date"} 150
comfoconnect_pdo_value{ID="130",description="Power Consumption: Total from start"} 150
comfoconnect_pdo_value{ID="144",description="Preheater Power Consumption: Total year-to-date"} 0
comfoconnect_pdo_value{ID="145",description="Preheater Power Consumption: Total from start"} 0
comfoconnect_pdo_value{ID="146",description="Preheater Power Consumption: Current Ventilation"} 0
comfoconnect_pdo_value{ID="16",description="Unknown"} 1
comfoconnect_pdo_value{ID="176",description="Unknown"} 0
comfoconnect_pdo_value{ID="192",description="Days left before filters must be replaced"} 4096
comfoconnect_pdo_value{ID="208",description="Unknown temperature"} 0
comfoconnect_pdo_value{ID="209",description="Current RMOT"} 151
comfoconnect_pdo_value{ID="210",description="Unknown"} 0
comfoconnect_pdo_value{ID="211",description="Unknown"} 0
comfoconnect_pdo_value{ID="212",description="Temperature profile: cool"} 217
comfoconnect_pdo_value{ID="213",description="Avoided Heating: Avoided actual"} 761
comfoconnect_pdo_value{ID="214",description="Avoided Heating: Avoided year-to-date"} 910
comfoconnect_pdo_value{ID="215",description="Avoided Heating: Avoided total"} 910
comfoconnect_pdo_value{ID="216",description="Avoided Cooling: Avoided actual"} 0
comfoconnect_pdo_value{ID="217",description="Avoided Cooling: Avoided year-to-date"} 44
comfoconnect_pdo_value{ID="218",description="Avoided Cooling: Avoided total"} 44
comfoconnect_pdo_value{ID="219",description="Unknown"} 0
comfoconnect_pdo_value{ID="220",description="Temperature: Outdoor Air"} 130
comfoconnect_pdo_value{ID="221",description="Temperature: Supply Air"} 206
comfoconnect_pdo_value{ID="224",description="Unknown"} 3
comfoconnect_pdo_value{ID="225",description="Unknown"} 1
comfoconnect_pdo_value{ID="226",description="Unknown"} 200
comfoconnect_pdo_value{ID="227",description="Bypass state"} 0
comfoconnect_pdo_value{ID="228",description="Unknown Frost Protection Unbalance"} 0
comfoconnect_pdo_value{ID="230",description="unknown"} 0
comfoconnect_pdo_value{ID="274",description="Temperature: Extract Air"} 215
comfoconnect_pdo_value{ID="275",description="Temperature: Exhaust Air"} 154
comfoconnect_pdo_value{ID="278",description="PostHeaterTempBefore"} -12800
comfoconnect_pdo_value{ID="290",description="Humidity: Extract Air"} 70
comfoconnect_pdo_value{ID="291",description="Humidity: Exhaust Air"} 84
comfoconnect_pdo_value{ID="292",description="Humidity: Outdoor Air"} 90
comfoconnect_pdo_value{ID="294",description="Humidity: Supply Air"} 70
comfoconnect_pdo_value{ID="321",description="Unknown"} 1
comfoconnect_pdo_value{ID="325",description="Unknown"} 1
comfoconnect_pdo_value{ID="33",description="Unknown"} 0
comfoconnect_pdo_value{ID="330",description="unknown"} 1
comfoconnect_pdo_value{ID="337",description="Unknown"} 0
comfoconnect_pdo_value{ID="338",description="Unknown"} 0
comfoconnect_pdo_value{ID="341",description="Unknown"} 0
comfoconnect_pdo_value{ID="345",description="unknown"} 0
comfoconnect_pdo_value{ID="346",description="unknown"} 0
comfoconnect_pdo_value{ID="369",description="Unknown"} 0
comfoconnect_pdo_value{ID="37",description="Unknown"} 0
comfoconnect_pdo_value{ID="370",description="Unknown"} 0
comfoconnect_pdo_value{ID="371",description="Unknown"} 0
comfoconnect_pdo_value{ID="372",description="Unknown"} 0
comfoconnect_pdo_value{ID="384",description="Unknown"} 65136
comfoconnect_pdo_value{ID="386",description="Unknown"} 0
comfoconnect_pdo_value{ID="400",description="Unknown"} 65136
comfoconnect_pdo_value{ID="401",description="Unknown"} 0
comfoconnect_pdo_value{ID="402",description="Unknown Post Heater Present"} 1
comfoconnect_pdo_value{ID="416",description="unknown Outdoor air temperature"} 28926
comfoconnect_pdo_value{ID="417",description="unknown GHE Ground temperature"} 25600
comfoconnect_pdo_value{ID="418",description="unknown GHE State"} 0
comfoconnect_pdo_value{ID="419",description="unknown GHE Present"} 1
comfoconnect_pdo_value{ID="42",description="Unknown"} 0
comfoconnect_pdo_value{ID="49",description="Operating mode1"} 255
comfoconnect_pdo_value{ID="53",description="Unknown"} 255
comfoconnect_pdo_value{ID="56",description="Operating mode2"} 255
comfoconnect_pdo_value{ID="57",description="Unknown"} 255
comfoconnect_pdo_value{ID="58",description="Unknown"} 255
comfoconnect_pdo_value{ID="65",description="Fans: Fan speed setting"} 2
comfoconnect_pdo_value{ID="66",description="Unknown"} 0
comfoconnect_pdo_value{ID="67",description="Unknown"} 1
comfoconnect_pdo_value{ID="70",description="Unknown"} 0
comfoconnect_pdo_value{ID="71",description="Unknown"} 0
comfoconnect_pdo_value{ID="73",description="Unknown"} 0
comfoconnect_pdo_value{ID="74",description="Unknown"} 0
comfoconnect_pdo_value{ID="81",description="General: Countdown until next fan speed change"} 4.294967295e+09
comfoconnect_pdo_value{ID="82",description="Unknown"} 4.294967295e+09
comfoconnect_pdo_value{ID="85",description="Unknown"} 4.294967295e+09
comfoconnect_pdo_value{ID="86",description="Unknown"} 4.294967295e+09
comfoconnect_pdo_value{ID="87",description="Unknown"} 4.294967295e+09
comfoconnect_pdo_value{ID="89",description="Unknown"} 4.294967295e+09
comfoconnect_pdo_value{ID="90",description="Unknown"} 4.294967295e+09```

