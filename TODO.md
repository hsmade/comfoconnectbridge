TODO
----

 * write config struct or parameters for cmd

 * implement a web API for simple control (fan speed, etc)

 * add makefile
   * write target for generating code for proto
   * write target for build, docker image, etc
 
 * [X] parse gateway operation messages using proto file
 
 * [X] map and decode R-PDOs
 
 * [ ] decode RMI? Does it contain useful data?
 
 * [ ] write tests for pkg/comfoconnect/*
 
 * [ ] design proxy/bridge that:
   * is a server to the app, handles registrations and sessions
   * forwards all other requests from the app to Lan-c with the uuid of our own client (fan-in)
   * copies all answers from lan-c to all connected apps (fan-out)
   * is a client to lan-c, requesting updates for every known PDO
   * keeps metrics for all see traffic per operation type
   * keep metrics for all PDOs received from Lan-c
   
   