TODO
----

 * write the client to connect to the comfoair unit
 * store a channel to forward data for all our clients on the bridge struct
 * forward all data messages from the comfoair unit to our clients through the bridge
 * implement a listener (maybe impersonating as a client) that extracts metrics and expose them through prometheus
 * investigate if we should/can forward config changes from the client(s) to comfoair, and how to proxy the response to that
 * write config struct or parameters for cmd
 * implement a web API for simple control (fan speed, etc)
 * add makefile
   * write target for generating code for proto
   * write target for build, docker image, etc
 * get comfoair version and store it somewhere, so we can answer properly on `VersionRequestType`
 * answer properly to `CnTimeRequestType`
 * ignore `i/o timeout` during read, it's normal because we've set a timeout, just `continue`
   
   