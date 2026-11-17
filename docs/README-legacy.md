# alloy

runtime for you and your agents

todo 

mvp
- [ ] make node a subscriber-emitter for communication
- [ ] clients package with no in house clients
- [ ] worker spawning with hot reload
- [ ] shared server and client gateway for a node
- [ ] better service abstraction for agents and humans to use

runtime conversion
- [ ] hot reloading
- [ ] more communication protocols 
- [ ] remote nodes
- [ ] language agnostic nodes
    - [ ] super imp to include shell languages
- [ ] mcp for projects to add node to the central store 

todo (archive)
- [x] logger
- [x] renaming concepts
- [x] encapsulate servcies
- [x] service pass logic
- [ ] notification channels (borrow concepts from calwbot)
- [ ] base triggers
    - [x] webhook (lisen using serverMux)
    - [x] polling (ticker + http do)
    - [x] tickers (prolly just use the default one)
    - [x] timed exec (just use time.Until in a timer)
- [x] error handling
- [x] better engine sync
- [x] trigger chainging <-
- [x] dispatcher
- [x] polish dispatcher
- [x] payload as btytes
- [x] ngrok tunneling
- [x] buffered jobs
- [x] buffered C's
- [x] custom services/ refactor services
- [ ] draw flow
- [ ] configurable port no
- [ ] clients
    - [x] notion
    - [x] ai
    - [ ] broswerbase
    - [ ] browseruse
- [ ] payload as byte 'stream?'
- [ ] ci
- [ ] optimized deployment pipeline for a common user
- [ ] better jobs logging and schema
    - timestamps
    - trace ID
    - retry count
    - metadata
- [ ] job based routing instead of source(node) based

my flows
- [ ] bang
- [x] nottager
- [ ] sum up HN
- [ ] sum up yt feed