# scdt

SCDT(single connection data transmission) is package of tcp data transmission

It use TCP to establish a one-way channel
And make both ends free to send and receive data

server:
```go
l, err := NewListener("0.0.0.0:12345")
if err != nil {
    panic(err)
}

//send some data to client and wait success
l.RangeConnections(f func(id string, connection Connection){
    l.SendTo(id, []byte("hello"), func(id string, message *Message){
         //do something
    })
}) 

//wait
time.Sleep(30*time.Minute)
//stop
l.Stop()
```

client
```go
dial, err := net.Dial("tcp", "localhost:12345")
if err != nil {
    log.Errorw("dail error", "err", err)
}
connect := Connect(dial)
//send some data to server and wait success
msg, b := connect.SendOnWait([]byte("hello"))
if b {
    fmt.Printf("waited send message:%+v,data:%s\n", msg, msg.Data)
}
//send some data to server and wait success callback
connect.SendWithCallback([]byte("hello"), func(message *Message) {
    fmt.Printf("send message:%+v,data:%s\n", message, message.Data)
})
//send some data to server
connect.Send([]byte("hello"))

//send some data to server with a custom id and wait success callback
connect.SendCustomDataWithCallback(0x01,[]byte("hello"), func(message *Message) {
    fmt.Printf("send message:%+v,data:%s\n", message, message.Data)		
})

//called when recv some message
connect.Recv(func(message *Message) ([]byte, error) {
    fmt.Printf("recv message:%+v,data:%s\n", id, message.Data)
    return nil, nil
})
```