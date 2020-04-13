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
   * is half a client to lan-c, requesting updates for every known PDO
   * keeps metrics for all see traffic per operation type
   * keep metrics for all PDOs received from Lan-c
   
   
       app -> [ => decode, metrics, encode with us as src                   => ] -> lan-c
           /  [ <= encode ,duplicate per app + replace dst, metrics, decode <= ]
          |    
       internal-client
          
       Listener-handler:
       type app struct {
            uuid []byte
            conn net.Conn
       }
       
       func (a *app) HandleConnection(remote chan message) {
            for {
                read message
                switch message.operationType
                case register: answer with confirm; store uuid (src)
                case start sessions: answer with confirm and 2 node-notifications
                default: remote <- message
            }
       }
                     
       proxy:
       type proxy struct {
            client Client
            uuid []byte
       }
       
       func (p Proxy) Run() {
            p.client := Client{IP: "x.x.x.x"}
            go client.Run()
            
            for {
                select {
                    case message <- p.toClient:
                        generateMetrics(message)
                        message.src = p.uuid
                        p.client.toRemote <- message
                    case message <- p.client.fromRemote:
                        generateMetrics(message)
                        for _, app := range p.apps {
                            message.dst = app.uuid
                            app.write(message.encode())
                        }                     
                }
            }
       } 
       
       client:
       type Client struct {
            IP string
            uuid []byte
            toRemote chan message
            fromRemote chan message
       }
       
       func (c Client) Run() {
            connect to c.ip
            send register app request
            send create session request
            send all PDO subscriptions  
            for {
                select {
                    case m := <- c.toRemote: conn.write(m.encode())
                    default: c.fromRemote <- readMessage()
                }
            }          
       }
       